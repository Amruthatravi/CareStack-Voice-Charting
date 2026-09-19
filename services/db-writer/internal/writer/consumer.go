package writer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"db-writer/internal/metrics"
)

const (
	stream         = "db_writes"
	consumerGroup  = "db-writer-group"
	consumerName   = "db-writer-1"
	maxRetries     = 3
	batchSize      = 10
)

type Consumer struct {
	rdb     *redis.Client
	db      *sql.DB
	logger  *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	metrics *metrics.Recorder
}

func NewConsumer(rdb *redis.Client, db *sql.DB, logger *slog.Logger, rec *metrics.Recorder) *Consumer {
	if rec == nil {
		rec = metrics.NewRecorder(500)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Consumer{
		rdb:     rdb,
		db:      db,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		metrics: rec,
	}
}

func (c *Consumer) Stop() {
	c.cancel()
}

func (c *Consumer) Run() error {
	// Create consumer group (OK if already exists)
	c.rdb.XGroupCreateMkStream(c.ctx, stream, consumerGroup, "0")

	c.logger.Info("DB-Writer listening", "stream", stream, "group", consumerGroup)

	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		results, err := c.rdb.XReadGroup(c.ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{stream, ">"},
			Count:    batchSize,
			Block:    2 * time.Second,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue // timeout, no new messages
			}
			if c.ctx.Err() != nil {
				return nil
			}
			c.logger.Error("XReadGroup error", "error", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, xStream := range results {
			for _, msg := range xStream.Messages {
				c.processMessage(msg)
			}
		}
	}
}

func (c *Consumer) processMessage(msg redis.XMessage) {
	vals := msg.Values

	tenantID, _ := vals["tenant_id"].(string)
	patientID, _ := vals["patient_id"].(string)
	visitID, _ := vals["visit_id"].(string)
	toothStr, _ := vals["tooth_num"].(string)
	surface, _ := vals["surface"].(string)
	eventType, _ := vals["event_type"].(string)
	measurementsStr, _ := vals["measurements"].(string)
	versionStr, _ := vals["version"].(string)
	sessionID, _ := vals["session_id"].(string)
	createdAtNsStr, _ := vals["created_at_ns"].(string)

	toothNum, _ := strconv.Atoi(toothStr)
	version, _ := strconv.ParseInt(versionStr, 10, 64)

	// createdAt is when voice-session finished parsing this event — the
	// clock we measure queue_wait and end_to_end latency against. Older
	// messages produced before this field existed won't have it; skip
	// timing rather than recording a bogus latency in that case.
	var createdAt time.Time
	if ns, err := strconv.ParseInt(createdAtNsStr, 10, 64); err == nil && ns > 0 {
		createdAt = time.Unix(0, ns)
		c.metrics.Record("queue_wait", time.Since(createdAt))
	}

	var measurements map[string]any
	_ = json.Unmarshal([]byte(measurementsStr), &measurements)

	var writeErr error
	dbWriteStart := time.Now()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		writeErr = c.upsertReading(tenantID, patientID, visitID, sessionID, toothNum, surface, eventType, measurements, version)
		if writeErr == nil {
			break
		}
		c.logger.Warn("Upsert failed, retrying", "attempt", attempt, "error", writeErr)
		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}
	c.metrics.Record("db_write", time.Since(dbWriteStart))
	if !createdAt.IsZero() {
		c.metrics.Record("end_to_end", time.Since(createdAt))
	}

	// Publish confirmation back to session's pub/sub channel
	confirmPayload, _ := json.Marshal(map[string]any{
		"type":    confirmType(writeErr),
		"version": version,
		"error":   errMsg(writeErr),
	})
	_ = c.rdb.Publish(c.ctx, fmt.Sprintf("ui:%s", sessionID), string(confirmPayload)).Err()

	if writeErr != nil {
		// Dead-letter the failed message
		c.deadLetter(msg, tenantID, eventType, measurements, writeErr)
		c.logger.Error("Permanent write failure — dead-lettered", "stream_id", msg.ID, "error", writeErr)
	}

	// ACK the message regardless (we've handled it or dead-lettered it)
	_ = c.rdb.XAck(c.ctx, stream, consumerGroup, msg.ID).Err()
}

func (c *Consumer) upsertReading(
	tenantID, patientID, visitID, sessionID string,
	toothNum int, surface, eventType string,
	measurements map[string]any,
	version int64,
) error {
	// Ensure session row exists
	_, err := c.db.ExecContext(c.ctx, `
		INSERT INTO periodontal_sessions (id, tenant_id, patient_id, visit_id, mock_mode)
		VALUES ($1::uuid, $2, $3, $4, false)
		ON CONFLICT (tenant_id, visit_id) DO NOTHING
	`, sessionID, tenantID, patientID, visitID)
	if err != nil {
		return fmt.Errorf("ensure session: %w", err)
	}

	// Get session UUID
	var dbSessionID string
	err = c.db.QueryRowContext(c.ctx,
		`SELECT id::text FROM periodontal_sessions WHERE tenant_id=$1 AND visit_id=$2`,
		tenantID, visitID,
	).Scan(&dbSessionID)
	if err != nil {
		return fmt.Errorf("get session id: %w", err)
	}

	if eventType != "measurement" {
		return nil // only persist measurements
	}

	// Extract measurement fields
	var pocketDepth []int
	if pd, ok := measurements["pocket_depth"]; ok {
		switch v := pd.(type) {
		case []interface{}:
			for _, x := range v {
				if f, ok := x.(float64); ok {
					pocketDepth = append(pocketDepth, int(f))
				}
			}
		case []int:
			pocketDepth = v
		}
	}

	var bleedingArr []bool
	if b, ok := measurements["bleeding"]; ok {
		if bBool, ok := b.(bool); ok && bBool {
			bleedingArr = []bool{true, true, true}
		} else {
			bleedingArr = []bool{false, false, false}
		}
	}

	var suppArr []bool
	if s, ok := measurements["suppuration"]; ok {
		if sBool, ok := s.(bool); ok && sBool {
			suppArr = []bool{true, true, true}
		}
	}

	toSQLArr := func(arr []int) interface{} {
		if arr == nil {
			return nil
		}
		return fmt.Sprintf("ARRAY[%d,%d,%d]", arr[0], arr[1], arr[2])
	}
	_ = toSQLArr

	// Build pocket depth array string for PostgreSQL
	pdSQL := "NULL"
	if len(pocketDepth) == 3 {
		pdSQL = fmt.Sprintf("ARRAY[%d,%d,%d]::integer[]", pocketDepth[0], pocketDepth[1], pocketDepth[2])
	}

	bleedSQL := "NULL"
	if len(bleedingArr) == 3 {
		bleedSQL = fmt.Sprintf("ARRAY[%v,%v,%v]::boolean[]", bleedingArr[0], bleedingArr[1], bleedingArr[2])
	}

	suppSQL := "NULL"
	if len(suppArr) == 3 {
		suppSQL = fmt.Sprintf("ARRAY[%v,%v,%v]::boolean[]", suppArr[0], suppArr[1], suppArr[2])
	}

	var recession *int
	if r, ok := measurements["recession"]; ok {
		if rf, ok := r.(float64); ok {
			ri := int(rf)
			recession = &ri
		}
	}

	var furcation *int
	if f, ok := measurements["furcation"]; ok {
		if ff, ok := f.(float64); ok {
			fi := int(ff)
			furcation = &fi
		}
	}

	var mobility *int
	if m, ok := measurements["mobility"]; ok {
		if mf, ok := m.(float64); ok {
			mi := int(mf)
			mobility = &mi
		}
	}

	query := fmt.Sprintf(`
		INSERT INTO periodontal_readings
			(session_id, tenant_id, patient_id, visit_id, tooth_number, surface,
			 pocket_depth, bleeding, suppuration, recession, furcation, mobility,
			 source, version, recorded_at)
		VALUES
			($1::uuid, $2, $3, $4, $5, $6,
			 %s, %s, %s, $7, $8, $9,
			 'voice', $10, NOW())
		ON CONFLICT (session_id, tooth_number, surface)
		DO UPDATE SET
			pocket_depth = COALESCE(EXCLUDED.pocket_depth, periodontal_readings.pocket_depth),
			bleeding     = COALESCE(EXCLUDED.bleeding, periodontal_readings.bleeding),
			suppuration  = COALESCE(EXCLUDED.suppuration, periodontal_readings.suppuration),
			recession    = COALESCE(EXCLUDED.recession, periodontal_readings.recession),
			furcation    = COALESCE(EXCLUDED.furcation, periodontal_readings.furcation),
			mobility     = COALESCE(EXCLUDED.mobility, periodontal_readings.mobility),
			version      = EXCLUDED.version,
			recorded_at  = NOW()
	`, pdSQL, bleedSQL, suppSQL)

	_, err = c.db.ExecContext(c.ctx, query,
		dbSessionID, tenantID, patientID, visitID, toothNum, surface,
		recession, furcation, mobility, version,
	)
	return err
}

func (c *Consumer) deadLetter(msg redis.XMessage, tenantID, eventType string, measurements map[string]any, writeErr error) {
	payloadJSON, _ := json.Marshal(measurements)
	_, _ = c.db.ExecContext(c.ctx, `
		INSERT INTO failed_events (tenant_id, stream_id, event_type, payload, error_message)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING
	`, tenantID, msg.ID, eventType, string(payloadJSON), writeErr.Error())
}

func confirmType(err error) string {
	if err == nil {
		return "db_confirmed"
	}
	return "db_failed"
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
