-- JWT jti denylist for Coach API token revocation (ADR-001).
-- Task 2a groundwork uses an in-memory Denylist; this table is the Postgres shape
-- for a later store implementation.

CREATE TABLE jwt_jti_denylist (
    jti        TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Compaction / expiry sweeps.
CREATE INDEX jwt_jti_denylist_expires_at_idx ON jwt_jti_denylist (expires_at);
