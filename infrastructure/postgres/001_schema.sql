-- CareStack Voice Periodontal Charting — Database Schema
-- HIPAA-compliant, multi-tenant, versioned periodontal records
-- Migration: 001_initial

-- ============================================================
-- Extensions
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "pgcrypto";  -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "btree_gin"; -- GIN index support

-- ============================================================
-- Tenants (practices)
-- ============================================================
CREATE TABLE IF NOT EXISTS tenants (
    id          VARCHAR(100) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE
);

-- ============================================================
-- Clinical Sessions
-- ============================================================
CREATE TABLE IF NOT EXISTS periodontal_sessions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     VARCHAR(100) NOT NULL REFERENCES tenants(id),
    patient_id    VARCHAR(100) NOT NULL,
    visit_id      VARCHAR(100) NOT NULL,
    clinician_id  VARCHAR(100),
    mock_mode     BOOLEAN      NOT NULL DEFAULT FALSE,
    started_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    ended_at      TIMESTAMPTZ,
    status        VARCHAR(20)  NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'completed', 'abandoned')),
    UNIQUE (tenant_id, visit_id)
);

CREATE INDEX idx_sessions_tenant    ON periodontal_sessions(tenant_id, started_at DESC);
CREATE INDEX idx_sessions_patient   ON periodontal_sessions(tenant_id, patient_id);

-- ============================================================
-- Periodontal Readings (upserted per tooth+surface per session)
-- ============================================================
CREATE TABLE IF NOT EXISTS periodontal_readings (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    UUID         NOT NULL REFERENCES periodontal_sessions(id),
    tenant_id     VARCHAR(100) NOT NULL,
    patient_id    VARCHAR(100) NOT NULL,
    visit_id      VARCHAR(100) NOT NULL,

    -- Tooth identification
    tooth_number  SMALLINT     NOT NULL CHECK (tooth_number BETWEEN 1 AND 32),
    surface       VARCHAR(10)  NOT NULL CHECK (surface IN ('buccal', 'lingual')),

    -- Primary periodontal measurements (3 per surface: mesial, mid, distal)
    pocket_depth  INTEGER[]    CHECK (array_length(pocket_depth, 1) = 3),
    bleeding      BOOLEAN[]    CHECK (array_length(bleeding, 1) = 3),
    suppuration   BOOLEAN[]    CHECK (array_length(suppuration, 1) = 3),
    plaque        BOOLEAN[]    CHECK (array_length(plaque, 1) = 3),

    -- Single-value measurements
    recession     SMALLINT     CHECK (recession BETWEEN 0 AND 20),
    furcation     SMALLINT     CHECK (furcation BETWEEN 1 AND 3),
    mobility      SMALLINT     CHECK (mobility BETWEEN 0 AND 3),

    -- Metadata
    source        VARCHAR(10)  NOT NULL DEFAULT 'voice'
                  CHECK (source IN ('voice', 'manual', 'import')),
    version       BIGINT       NOT NULL DEFAULT 0,
    confidence    NUMERIC(4,3) CHECK (confidence BETWEEN 0 AND 1),
    recorded_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- One row per tooth+surface per session (upsert target)
    UNIQUE (session_id, tooth_number, surface)
);

CREATE INDEX idx_readings_session   ON periodontal_readings(session_id);
CREATE INDEX idx_readings_tooth     ON periodontal_readings(session_id, tooth_number);
CREATE INDEX idx_readings_tenant    ON periodontal_readings(tenant_id, patient_id, recorded_at DESC);

-- ============================================================
-- Audit Log (immutable, append-only per HIPAA §164.312(b))
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_log (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     VARCHAR(100) NOT NULL,
    session_id    UUID,
    event_type    VARCHAR(50)  NOT NULL,
    event_data    JSONB,
    clinician_id  VARCHAR(100),
    patient_id    VARCHAR(100),
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_tenant       ON audit_log(tenant_id, created_at DESC);
CREATE INDEX idx_audit_session      ON audit_log(session_id);
CREATE INDEX idx_audit_patient      ON audit_log(tenant_id, patient_id);
CREATE INDEX idx_audit_event        ON audit_log USING GIN (event_data);

-- Prevent DELETE/UPDATE on audit_log (simulate WORM)
CREATE RULE audit_log_no_delete AS ON DELETE TO audit_log DO INSTEAD NOTHING;
CREATE RULE audit_log_no_update AS ON UPDATE TO audit_log DO INSTEAD NOTHING;

-- ============================================================
-- Redis Stream Dead-Letter Queue tracking (for ops visibility)
-- ============================================================
CREATE TABLE IF NOT EXISTS failed_events (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     VARCHAR(100) NOT NULL,
    stream_id     VARCHAR(255) NOT NULL,  -- Redis stream message ID
    event_type    VARCHAR(50),
    payload       JSONB,
    error_message TEXT,
    retry_count   SMALLINT     NOT NULL DEFAULT 0,
    first_failed  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_failed   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ
);

-- ============================================================
-- Demo seed data
-- ============================================================
INSERT INTO tenants (id, name) VALUES
    ('demo-practice', 'CareStack Demo Practice')
ON CONFLICT DO NOTHING;
