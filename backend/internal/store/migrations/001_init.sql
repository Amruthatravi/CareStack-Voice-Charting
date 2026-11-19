CREATE TABLE IF NOT EXISTS sessions (
    id            UUID PRIMARY KEY,
    patient_label TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Append-only audit log: one row per spoken utterance, never updated or deleted.
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    event_id    UUID NOT NULL UNIQUE,
    session_id  UUID NOT NULL REFERENCES sessions(id),
    seq         BIGINT NOT NULL,
    kind        TEXT NOT NULL,
    raw_text    TEXT NOT NULL,
    source      TEXT NOT NULL,
    latency_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
    commands    JSONB NOT NULL DEFAULT '[]',
    changes     JSONB NOT NULL DEFAULT '[]',
    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_log_session_seq ON audit_log (session_id, seq);

CREATE OR REPLACE FUNCTION audit_log_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_log_no_mutate ON audit_log;
CREATE TRIGGER audit_log_no_mutate BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
DROP TRIGGER IF EXISTS audit_log_no_truncate ON audit_log;
CREATE TRIGGER audit_log_no_truncate BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_immutable();

-- Latest materialised chart per session (rebuildable from audit_log).
CREATE TABLE IF NOT EXISTS chart_snapshots (
    session_id UUID PRIMARY KEY REFERENCES sessions(id),
    chart      JSONB NOT NULL,
    last_seq   BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
