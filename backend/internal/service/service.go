// Package service ties it together: parse -> (LLM slow path) -> apply -> persist state in Redis -> publish event.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"perioassist/internal/chart"
	"perioassist/internal/event"
	"perioassist/internal/llm"
	"perioassist/internal/parser"
	"perioassist/internal/store"
	"perioassist/internal/stream"
)

type Meta struct {
	Path      string          `json:"path"` // init | regex | llm | fallback
	LatencyMS float64         `json:"latencyMs"`
	Messages  []string        `json:"messages"`
	Commands  []chart.Command `json:"commands"`
	EventID   string          `json:"eventId,omitempty"`
	Seq       int64           `json:"seq"`
	Heard     string          `json:"heard"`
}

type Response struct {
	SessionID string       `json:"sessionId"`
	Chart     *chart.Chart `json:"chart"`
	Meta      Meta         `json:"meta"`
}

type Service struct {
	rdb   *redis.Client
	db    *store.Store
	llm   *llm.Client
	prod  *stream.Producer
	locks sync.Map
}

func New(rdb *redis.Client, db *store.Store, l *llm.Client, p *stream.Producer) *Service {
	return &Service{rdb: rdb, db: db, llm: l, prod: p}
}

func (s *Service) LLMEnabled() bool { return s.llm.Enabled() }

type persisted struct {
	Chart   *chart.Chart  `json:"chart"`
	History []chart.Entry `json:"history"`
}

func (s *Service) lock(id string) *sync.Mutex {
	m, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return m.(*sync.Mutex)
}

func (s *Service) load(ctx context.Context, sid string) (*chart.Chart, error) {
	raw, err := s.rdb.Get(ctx, "chart:"+sid).Bytes()
	if err == nil {
		var p persisted
		if json.Unmarshal(raw, &p) == nil && p.Chart != nil {
			p.Chart.History = p.History
			if p.Chart.Teeth == nil {
				p.Chart.Teeth = map[int]*chart.Tooth{}
			}
			return p.Chart, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		return nil, err
	}
	// Redis cold (restart / eviction): rebuild from the last Postgres snapshot.
	if snap, err := s.db.LoadSnapshot(ctx, sid); err == nil && snap != nil {
		var c chart.Chart
		if json.Unmarshal(snap, &c) == nil {
			if c.Teeth == nil {
				c.Teeth = map[int]*chart.Tooth{}
			}
			return &c, nil
		}
	}
	return chart.New(), nil
}

func (s *Service) save(ctx context.Context, sid string, c *chart.Chart) error {
	b, err := json.Marshal(persisted{Chart: c, History: c.History})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, "chart:"+sid, b, 48*time.Hour).Err()
}

// State returns the current chart without changing it.
func (s *Service) State(ctx context.Context, sid string) (*Response, error) {
	mu := s.lock(sid)
	mu.Lock()
	defer mu.Unlock()
	c, err := s.load(ctx, sid)
	if err != nil {
		return nil, err
	}
	return &Response{SessionID: sid, Chart: c, Meta: Meta{Path: "init", Messages: []string{}}}, nil
}

func hintFor(c *chart.Chart) llm.Hint {
	h := llm.Hint{CursorTooth: c.Cursor.Tooth}
	if c.Last != nil {
		h.HasLast, h.LastTooth, h.LastSite, h.LastSurface = true, c.Last.Tooth, c.Last.Site+1, string(c.Last.Surface)
		if t, ok := c.Teeth[c.Last.Tooth]; ok {
			sd := t.Buccal
			if c.Last.Surface == chart.Lingual {
				sd = t.Lingual
			}
			h.LastPD = sd.PD[c.Last.Site]
		}
	}
	return h
}

func toCommand(in *llm.Interpretation) *chart.Command {
	val := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	switch in.Action {
	case "set_pd":
		return &chart.Command{Kind: chart.KSetSite, Tooth: val(in.Tooth), Site: val(in.Site), Value: val(in.Value)}
	case "set_recession":
		return &chart.Command{Kind: chart.KRecession, Tooth: val(in.Tooth), Site: val(in.Site), Value: val(in.Value)}
	case "set_flag":
		return &chart.Command{Kind: chart.KFlag, Tooth: val(in.Tooth), Site: val(in.Site), Flag: in.Flag}
	case "goto":
		return &chart.Command{Kind: chart.KGoto, Tooth: val(in.Tooth)}
	case "undo":
		return &chart.Command{Kind: chart.KUndo}
	}
	return nil
}

// Handle processes one finalised utterance for a session.
func (s *Service) Handle(ctx context.Context, sid, text string) (*Response, error) {
	start := time.Now()
	text = strings.TrimSpace(text)

	mu := s.lock(sid)
	mu.Lock()
	defer mu.Unlock()

	c, err := s.load(ctx, sid)
	if err != nil {
		return nil, err
	}

	res := parser.Parse(text)
	path := "regex"
	cmds := res.Commands
	msgs := []string{}

	if parser.NeedsLLM(res) {
		lctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		in, err := s.llm.Interpret(lctx, sid, text, hintFor(c))
		cancel()
		switch {
		case err == nil:
			path = "llm"
			cmds = nil
			if cmd := toCommand(in); cmd != nil {
				cmds = []chart.Command{*cmd}
			} else {
				msgs = append(msgs, "couldn't interpret that correction")
			}
		default:
			if !errors.Is(err, llm.ErrDisabled) && !errors.Is(err, llm.ErrRateLimited) {
				log.Printf("llm: %v", err)
			}
			path = "fallback"
			if h := parser.Heuristic(res); len(h) > 0 {
				cmds = h
			}
		}
	}

	var changes []chart.Change
	kind := "noop"
	for _, cmd := range cmds {
		e, msg, err := c.Apply(cmd)
		if err != nil {
			msgs = append(msgs, err.Error())
			continue
		}
		if msg != "" {
			msgs = append(msgs, msg)
		}
		if cmd.Kind == chart.KUndo {
			kind = "undo"
		} else if kind == "noop" {
			kind = "nav"
		}
		if e != nil && len(e.Changes) > 0 {
			changes = append(changes, e.Changes...)
			if kind != "undo" {
				kind = "entry"
			}
		}
	}
	if len(cmds) == 0 && len(msgs) == 0 && text != "" {
		msgs = append(msgs, "didn't catch that")
	}

	if err := s.save(ctx, sid, c); err != nil {
		return nil, err
	}
	seq, err := s.rdb.Incr(ctx, "seq:"+sid).Result()
	if err != nil {
		return nil, err
	}

	ev := event.Event{ID: event.NewID(), SessionID: sid, Seq: seq, Kind: kind, Raw: text, Source: path, At: time.Now().UTC()}
	ev.Commands, _ = json.Marshal(cmds)
	if changes == nil {
		changes = []chart.Change{}
	}
	ev.Changes, _ = json.Marshal(changes)
	ev.Chart, _ = json.Marshal(c)
	lat := float64(time.Since(start).Microseconds()) / 1000
	ev.LatencyMS = lat
	if err := s.prod.Publish(ctx, ev); err != nil {
		return nil, err
	}

	if cmds == nil {
		cmds = []chart.Command{}
	}
	return &Response{SessionID: sid, Chart: c, Meta: Meta{
		Path: path, LatencyMS: lat, Messages: msgs, Commands: cmds, EventID: ev.ID, Seq: seq, Heard: text,
	}}, nil
}
