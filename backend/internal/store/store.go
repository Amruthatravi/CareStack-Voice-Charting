package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"perioassist/internal/event"
)

//go:embed migrations/001_init.sql
var initSQL string

type Store struct{ db *sql.DB }

func Open(ctx context.Context, url string) (*Store, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	var last error
	for i := 0; i < 30; i++ { // wait for postgres to come up
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		last = db.PingContext(pctx)
		cancel()
		if last == nil {
			return &Store{db: db}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("postgres not reachable: %w", last)
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(727274)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, initSQL); err != nil {
		return err
	}
	return tx.Commit()
}

type Session struct {
	ID           string    `json:"id"`
	PatientLabel string    `json:"patientLabel"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Store) CreateSession(ctx context.Context, label string) (*Session, error) {
	id := event.NewID()
	var created time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sessions (id, patient_label) VALUES ($1, $2) RETURNING created_at`, id, label).Scan(&created)
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, PatientLabel: label, CreatedAt: created}, nil
}

func (s *Store) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, patient_label, created_at FROM sessions ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var x Session
		if err := rows.Scan(&x.ID, &x.PatientLabel, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) SessionExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`, id).Scan(&ok)
	return ok, err
}

// LoadSnapshot returns the last persisted chart JSON, or nil if none.
func (s *Store) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT chart FROM chart_snapshots WHERE session_id = $1`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return b, err
}

func nz(b json.RawMessage, d string) string {
	if len(b) == 0 {
		return d
	}
	return string(b)
}

// WriteBatch appends events to the audit log and refreshes snapshots, atomically.
// It is idempotent (event_id is UNIQUE), so Redis redelivery is safe.
func (s *Store) WriteBatch(ctx context.Context, evs []event.Event) error {
	if len(evs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	newest := map[string]event.Event{}
	for _, ev := range evs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id) VALUES ($1) ON CONFLICT DO NOTHING`, ev.SessionID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO audit_log (event_id, session_id, seq, kind, raw_text, source, latency_ms, commands, changes, occurred_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10)
			ON CONFLICT (event_id) DO NOTHING`,
			ev.ID, ev.SessionID, ev.Seq, ev.Kind, ev.Raw, ev.Source, ev.LatencyMS,
			nz(ev.Commands, "[]"), nz(ev.Changes, "[]"), ev.At); err != nil {
			return err
		}
		if cur, ok := newest[ev.SessionID]; !ok || ev.Seq > cur.Seq {
			newest[ev.SessionID] = ev
		}
	}
	for sid, ev := range newest {
		if len(ev.Chart) == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chart_snapshots (session_id, chart, last_seq) VALUES ($1, $2::jsonb, $3)
			ON CONFLICT (session_id) DO UPDATE
			   SET chart = EXCLUDED.chart, last_seq = EXCLUDED.last_seq, updated_at = now()
			 WHERE chart_snapshots.last_seq < EXCLUDED.last_seq`, sid, string(ev.Chart), ev.Seq); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type AuditRow struct {
	ID         int64           `json:"id"`
	EventID    string          `json:"eventId"`
	Seq        int64           `json:"seq"`
	Kind       string          `json:"kind"`
	Raw        string          `json:"raw"`
	Source     string          `json:"source"`
	LatencyMS  float64         `json:"latencyMs"`
	Commands   json.RawMessage `json:"commands"`
	Changes    json.RawMessage `json:"changes"`
	OccurredAt time.Time       `json:"occurredAt"`
}

func (s *Store) ListAudit(ctx context.Context, sid string, limit int) ([]AuditRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_id, seq, kind, raw_text, source, latency_ms, commands, changes, occurred_at
		  FROM audit_log WHERE session_id = $1 ORDER BY seq DESC LIMIT $2`, sid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var r AuditRow
		var cmds, chg []byte
		if err := rows.Scan(&r.ID, &r.EventID, &r.Seq, &r.Kind, &r.Raw, &r.Source, &r.LatencyMS, &cmds, &chg, &r.OccurredAt); err != nil {
			return nil, err
		}
		r.Commands, r.Changes = cmds, chg
		out = append(out, r)
	}
	return out, rows.Err()
}
