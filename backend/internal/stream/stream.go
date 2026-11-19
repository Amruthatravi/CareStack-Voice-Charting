// Package stream moves events through Redis Streams. The request path only does an
// XADD (sub-millisecond); a consumer-group worker drains the stream into Postgres.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"perioassist/internal/event"
)

const (
	Stream     = "perio:events"
	DeadStream = "perio:events:dead"
	Group      = "pg-writers"
)

type Producer struct{ rdb *redis.Client }

func NewProducer(rdb *redis.Client) *Producer { return &Producer{rdb: rdb} }

func (p *Producer) Publish(ctx context.Context, ev event.Event) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: Stream, MaxLen: 200000, Approx: true,
		Values: map[string]any{"payload": string(b)},
	}).Err()
}

type Handler func(ctx context.Context, evs []event.Event) error

type Consumer struct {
	rdb  *redis.Client
	name string
	fn   Handler
}

func NewConsumer(rdb *redis.Client, name string, fn Handler) *Consumer {
	return &Consumer{rdb: rdb, name: name, fn: fn}
}

func (c *Consumer) Run(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, Stream, Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	c.drainPending(ctx)
	lastClaim := time.Now()
	for ctx.Err() == nil {
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: Group, Consumer: c.name, Streams: []string{Stream, ">"},
			Count: 100, Block: 2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				// idle
			} else {
				log.Printf("stream: read error: %v", err)
				time.Sleep(time.Second)
			}
		}
		for _, s := range res {
			c.process(ctx, s.Messages)
		}
		if time.Since(lastClaim) > 20*time.Second {
			c.claimStale(ctx)
			lastClaim = time.Now()
		}
	}
	return ctx.Err()
}

// drainPending re-processes messages this consumer read before a crash.
func (c *Consumer) drainPending(ctx context.Context) {
	for ctx.Err() == nil {
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: Group, Consumer: c.name, Streams: []string{Stream, "0"}, Count: 100, Block: -1,
		}).Result()
		if err != nil || len(res) == 0 || len(res[0].Messages) == 0 {
			return
		}
		if !c.process(ctx, res[0].Messages) {
			return
		}
	}
}

// claimStale takes over messages stuck on dead consumers or earlier failed writes.
func (c *Consumer) claimStale(ctx context.Context) {
	msgs, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: Stream, Group: Group, Consumer: c.name, MinIdle: 30 * time.Second, Start: "0-0", Count: 100,
	}).Result()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("stream: autoclaim: %v", err)
		}
		return
	}
	if len(msgs) > 0 {
		c.process(ctx, msgs)
	}
}

// process decodes, writes and acks. It returns false if the write failed (messages stay pending).
func (c *Consumer) process(ctx context.Context, msgs []redis.XMessage) bool {
	var evs []event.Event
	var ids []string
	for _, m := range msgs {
		var ev event.Event
		s, _ := m.Values["payload"].(string)
		if err := json.Unmarshal([]byte(s), &ev); err != nil || ev.ID == "" || ev.SessionID == "" {
			log.Printf("stream: dead-lettering %s: %v", m.ID, err)
			_ = c.rdb.XAdd(ctx, &redis.XAddArgs{Stream: DeadStream, MaxLen: 10000, Approx: true, Values: map[string]any{"id": m.ID, "payload": s}}).Err()
			_ = c.rdb.XAck(ctx, Stream, Group, m.ID).Err()
			continue
		}
		evs = append(evs, ev)
		ids = append(ids, m.ID)
	}
	if len(evs) == 0 {
		return true
	}
	if err := c.fn(ctx, evs); err != nil {
		log.Printf("stream: write failed (will retry): %v", err)
		return false
	}
	if err := c.rdb.XAck(ctx, Stream, Group, ids...).Err(); err != nil {
		log.Printf("stream: ack failed: %v", err)
	}
	return true
}
