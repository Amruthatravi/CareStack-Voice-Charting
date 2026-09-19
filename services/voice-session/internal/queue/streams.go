package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"voice-session/internal/parser"
)

type Producer struct {
	client *redis.Client
}

func NewProducer(client *redis.Client) *Producer {
	return &Producer{client: client}
}

func (p *Producer) Enqueue(ctx context.Context, event *parser.ChartEvent) error {
	measurementsJSON, err := json.Marshal(event.Measurements)
	if err != nil {
		return err
	}

	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: "db_writes",
		MaxLen: 10000,
		Values: map[string]interface{}{
			"session_id":     event.SessionID,
			"tenant_id":      event.TenantID,
			"patient_id":     event.PatientID,
			"visit_id":       event.VisitID,
			"tooth_num":      fmt.Sprintf("%d", event.ToothNum),
			"surface":        event.Surface,
			"event_type":     event.EventType,
			"measurements":   string(measurementsJSON),
			"version":        fmt.Sprintf("%d", event.Version),
			"ts":             fmt.Sprintf("%d", event.Timestamp.Unix()),
			"created_at_ns":  fmt.Sprintf("%d", event.Timestamp.UnixNano()), // nanosecond precision, used for latency measurement in db-writer
		},
	}).Err()
}

func (p *Producer) EnqueueConfirmation(ctx context.Context, sessionID string, version int64, success bool, errMsg string) error {
	successStr := "false"
	if success {
		successStr = "true"
	}

	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: "db_confirmations",
		Values: map[string]interface{}{
			"session_id": sessionID,
			"version":    fmt.Sprintf("%d", version),
			"success":    successStr,
			"error":      errMsg,
		},
	}).Err()
}
