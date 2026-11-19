// Package llm is the slow path: ambiguous spoken corrections go to Groq
// (Llama 3 8B). It is deliberately defensive so it never trips Groq's rate limits:
//
//   - only ambiguous utterances reach it (the regex fast path handles the rest)
//   - global token bucket + per-session minimum gap
//   - answers are cached by (text, cursor context)
//   - 429 -> circuit opens for Retry-After; callers fall back to a deterministic heuristic
//   - hard timeout, max_tokens capped, no retries
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrDisabled    = errors.New("llm disabled (no GROQ_API_KEY)")
	ErrRateLimited = errors.New("llm rate budget exhausted")
	ErrBadOutput   = errors.New("llm returned an invalid command")
)

type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
	RPM     int
	MinGap  time.Duration
}

type Hint struct {
	CursorTooth int    `json:"cursorTooth"`
	HasLast     bool   `json:"hasLast"`
	LastTooth   int    `json:"lastTooth,omitempty"`
	LastSite    int    `json:"lastSite,omitempty"` // 1..3
	LastSurface string `json:"lastSurface,omitempty"`
	LastPD      int    `json:"lastPD,omitempty"`
}

type Interpretation struct {
	Action string `json:"action"` // set_pd | set_flag | set_recession | goto | undo | none
	Tooth  *int   `json:"tooth"`
	Site   *int   `json:"site"`
	Value  *int   `json:"value"`
	Flag   string `json:"flag"`
}

type cached struct {
	in  Interpretation
	exp time.Time
}

type Client struct {
	cfg  Config
	hc   *http.Client
	mu   sync.Mutex
	tok  float64
	last time.Time
	open time.Time // circuit open until
	gap  map[string]time.Time
	memo map[string]cached
}

func New(cfg Config) *Client {
	if cfg.RPM <= 0 {
		cfg.RPM = 20
	}
	return &Client{
		cfg:  cfg,
		hc:   &http.Client{Timeout: cfg.Timeout},
		tok:  math.Min(float64(cfg.RPM), 10),
		last: time.Now(),
		gap:  map[string]time.Time{},
		memo: map[string]cached{},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.cfg.APIKey != "" }

func (c *Client) admit(session string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.Before(c.open) {
		return ErrRateLimited
	}
	burst := math.Min(float64(c.cfg.RPM), 10)
	c.tok = math.Min(burst, c.tok+now.Sub(c.last).Seconds()*float64(c.cfg.RPM)/60.0)
	c.last = now
	if t, ok := c.gap[session]; ok && now.Sub(t) < c.cfg.MinGap {
		return ErrRateLimited
	}
	if c.tok < 1 {
		return ErrRateLimited
	}
	c.tok--
	c.gap[session] = now
	if len(c.gap) > 2000 {
		c.gap = map[string]time.Time{session: now}
	}
	return nil
}

const systemPrompt = `You convert ONE spoken correction from a dental hygienist doing periodontal charting into ONE JSON object. Reply with JSON only.
Schema: {"action":"set_pd"|"set_flag"|"set_recession"|"goto"|"undo"|"none","tooth":int|null,"site":1|2|3|null,"value":int|null,"flag":"bop"|"supp"|"plaque"|null}
Rules:
- Pocket depths and recession are whole numbers 1-15. Sites are 1,2,3 on a tooth.
- "that", "last", "it" mean the most recently charted site (see context): leave tooth and site null.
- Spoken number words must become digits ("a five" -> 5).
- Use "undo" for "scratch that / cancel that". Use "none" when unsure. Never invent values.
Examples:
"no wait that was a five" -> {"action":"set_pd","tooth":null,"site":null,"value":5,"flag":null}
"sorry site two should be a four" -> {"action":"set_pd","tooth":null,"site":2,"value":4,"flag":null}
"actually tooth 14 site 3 is bleeding" -> {"action":"set_flag","tooth":14,"site":3,"value":null,"flag":"bop"}`

type chatReq struct {
	Model          string              `json:"model"`
	Messages       []map[string]string `json:"messages"`
	Temperature    float64             `json:"temperature"`
	MaxTokens      int                 `json:"max_tokens"`
	ResponseFormat map[string]string   `json:"response_format"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) Interpret(ctx context.Context, session, text string, h Hint) (*Interpretation, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	hb, _ := json.Marshal(h)
	key := strings.ToLower(strings.TrimSpace(text)) + "|" + string(hb)

	c.mu.Lock()
	if m, ok := c.memo[key]; ok && time.Now().Before(m.exp) {
		c.mu.Unlock()
		in := m.in
		return &in, nil
	}
	c.mu.Unlock()

	if err := c.admit(session); err != nil {
		return nil, err
	}

	body, _ := json.Marshal(chatReq{
		Model: c.cfg.Model,
		Messages: []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf("Context: %s\nUtterance: %q", hb, text)},
		},
		Temperature:    0,
		MaxTokens:      80,
		ResponseFormat: map[string]string{"type": "json_object"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode == http.StatusTooManyRequests {
		wait := 30 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			wait = time.Duration(s) * time.Second
		}
		c.mu.Lock()
		c.open = time.Now().Add(wait)
		c.mu.Unlock()
		return nil, ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq status %d", resp.StatusCode)
	}

	var cr chatResp
	if err := json.Unmarshal(raw, &cr); err != nil || len(cr.Choices) == 0 {
		return nil, ErrBadOutput
	}
	var in Interpretation
	if err := json.Unmarshal([]byte(cr.Choices[0].Message.Content), &in); err != nil {
		return nil, ErrBadOutput
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if len(c.memo) > 512 {
		c.memo = map[string]cached{}
	}
	c.memo[key] = cached{in: in, exp: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()
	return &in, nil
}

// Validate is a strict whitelist: the model can never produce anything the chart can't safely apply.
func (in *Interpretation) Validate() error {
	in.Action = strings.ToLower(strings.TrimSpace(in.Action))
	in.Flag = strings.ToLower(strings.TrimSpace(in.Flag))
	inRange := func(p *int, lo, hi int) bool { return p == nil || (*p >= lo && *p <= hi) }
	if !inRange(in.Tooth, 1, 32) || !inRange(in.Site, 1, 3) || !inRange(in.Value, 0, 15) {
		return ErrBadOutput
	}
	switch in.Action {
	case "set_pd":
		if in.Value == nil || *in.Value < 1 {
			return ErrBadOutput
		}
	case "set_recession":
		if in.Value == nil {
			return ErrBadOutput
		}
	case "set_flag":
		if in.Flag != "bop" && in.Flag != "supp" && in.Flag != "plaque" {
			return ErrBadOutput
		}
	case "goto":
		if in.Tooth == nil {
			return ErrBadOutput
		}
	case "undo", "none":
	default:
		return ErrBadOutput
	}
	return nil
}
