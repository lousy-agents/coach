-- Initial coach-api schema: jobs, attempt-scoped findings/diagnostics.
-- Postgres 16+: UNIQUE NULLS NOT DISTINCT so deterministic rows (null rubric_id)
-- cannot duplicate within the same (job_id, attempt, source, payload_hash).

CREATE TABLE jobs (
    id                   UUID PRIMARY KEY,
    kind                 TEXT NOT NULL,
    params               JSONB NOT NULL,
    status               TEXT NOT NULL,
    error                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at           TIMESTAMPTZ,
    finished_at          TIMESTAMPTZ,
    claimed_by           TEXT,
    heartbeat_at         TIMESTAMPTZ,
    attempt              INT NOT NULL DEFAULT 0,
    created_by_provider  TEXT NOT NULL,
    created_by_subject   TEXT NOT NULL,
    created_by_login     TEXT NOT NULL
);

-- Requeue reconciler / claim scans (status + age) and stale-heartbeat reclaim.
CREATE INDEX jobs_status_created_at_idx ON jobs (status, created_at);
CREATE INDEX jobs_running_heartbeat_at_idx ON jobs (heartbeat_at)
    WHERE status = 'running';

CREATE TABLE job_findings (
    id              UUID PRIMARY KEY,
    job_id          UUID NOT NULL REFERENCES jobs (id),
    attempt         INT NOT NULL,
    source          TEXT NOT NULL,
    rubric_id       TEXT,
    rubric_version  TEXT,
    model_identity  TEXT,
    payload         JSONB NOT NULL,
    payload_hash    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (job_id, attempt, source, rubric_id, payload_hash),
    CONSTRAINT job_findings_provenance_chk CHECK (
        (source = 'deterministic'
            AND rubric_id IS NULL
            AND rubric_version IS NULL
            AND model_identity IS NULL)
        OR
        (source = 'agent'
            AND rubric_id IS NOT NULL
            AND rubric_version IS NOT NULL
            AND model_identity IS NOT NULL)
    )
);

CREATE INDEX job_findings_job_id_idx ON job_findings (job_id);

CREATE TABLE job_diagnostics (
    id          UUID PRIMARY KEY,
    job_id      UUID NOT NULL REFERENCES jobs (id),
    attempt     INT NOT NULL,
    scope       TEXT NOT NULL,
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX job_diagnostics_job_id_idx ON job_diagnostics (job_id);
