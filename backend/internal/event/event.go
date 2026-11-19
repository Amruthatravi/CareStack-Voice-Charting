// Package event defines the record that flows through Redis Streams into the
// append-only Postgres audit log.
package event

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	SessionID string          `json:"sessionId"`
	Seq       int64           `json:"seq"`
	Kind      string          `json:"kind"` // entry | undo | nav | noop
	Raw       string          `json:"raw"`
	Source    string          `json:"source"` // regex | llm | fallback
	LatencyMS float64         `json:"latencyMs"`
	Commands  json.RawMessage `json:"commands"`
	Changes   json.RawMessage `json:"changes"`
	Chart     json.RawMessage `json:"chart"` // snapshot after the event
	At        time.Time       `json:"at"`
}

func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
