// Package postgres is the durable coachapi.WorkerJobStore backed by
// Postgres; its schema is internal/coachapi/migrations.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// Store is the durable JobStore backed by Postgres (schema in
// internal/coachapi/migrations). It uses the native pgx/v5 pool rather than
// database/sql: scanning nullable columns straight into Job's existing
// *string/*time.Time fields via double-pointer targets, and jsonb columns
// straight into json.RawMessage, are both pgx-native conveniences that
// database/sql's driver.Value interface does not offer as directly.
type Store struct {
	pool *pgxpool.Pool
}

var _ coachapi.JobStore = (*Store)(nil)

var _ coachapi.WorkerJobStore = (*Store)(nil)

// NewStore returns a JobStore backed by pool. The caller owns pool's
// lifecycle (construction and Close); Store never closes it.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// fenceHoldForTest, when non-nil, runs before atomic fenced inserts so
// acceptance tests can interleave ClaimJob reclaim. nil outside tests.
var fenceHoldForTest func()

func (s *Store) CreateJob(ctx context.Context, job coachapi.Job) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jobs (
			id, kind, params, status, error, created_at, started_at,
			finished_at, claimed_by, heartbeat_at, attempt,
			created_by_provider, created_by_subject, created_by_login
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		job.ID, string(job.Kind), job.Params, string(job.Status), job.Error,
		job.CreatedAt, job.StartedAt, job.FinishedAt, job.ClaimedBy,
		job.HeartbeatAt, job.Attempt, job.CreatedByProvider,
		job.CreatedBySubject, job.CreatedByLogin,
	)
	if err != nil {
		return fmt.Errorf("coachapi: create job %q: %w", job.ID, err)
	}
	return nil
}

func (s *Store) GetJob(ctx context.Context, id string) (coachapi.Job, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, kind, params, status, error, created_at, started_at,
			finished_at, claimed_by, heartbeat_at, attempt,
			created_by_provider, created_by_subject, created_by_login
		FROM jobs WHERE id = $1`, id)
	return scanJob(row)
}

func scanJob(row pgx.Row) (coachapi.Job, error) {
	var (
		job    coachapi.Job
		kind   string
		status string
	)
	err := row.Scan(
		&job.ID, &kind, &job.Params, &status, &job.Error, &job.CreatedAt,
		&job.StartedAt, &job.FinishedAt, &job.ClaimedBy, &job.HeartbeatAt,
		&job.Attempt, &job.CreatedByProvider, &job.CreatedBySubject,
		&job.CreatedByLogin,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coachapi.Job{}, fmt.Errorf("coachapi: job: %w", coachapi.ErrJobNotFound)
		}
		return coachapi.Job{}, fmt.Errorf("coachapi: get job: %w", err)
	}
	job.Kind = coachapi.JobKind(kind)
	job.Status = coachapi.JobStatus(status)
	return job, nil
}

func (s *Store) RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = $1, error = $2, finished_at = $3 WHERE id = $4`,
		string(coachapi.JobStatusFailed), errMsg, finishedAt, jobID,
	)
	if err != nil {
		return fmt.Errorf("coachapi: record failure: updating job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	return nil
}
