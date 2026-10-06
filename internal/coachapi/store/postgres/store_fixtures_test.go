package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/postgres"
)

func pgQueuedJob(id string) coachapi.Job {
	return coachapi.Job{
		ID:                id,
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets","ref":"main","extra":{"nested":[1,2,3],"flag":true}}`),
		Status:            coachapi.JobStatusQueued,
		CreatedAt:         time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		Attempt:           0,
		CreatedByProvider: "github",
		CreatedBySubject:  "12345",
		CreatedByLogin:    "octocat",
	}
}

// pgMigrationFiles returns every internal/coachapi/migrations/*.sql path, in
// filename order (0001_..., 0002_..., ...), read from disk rather than
// hand-duplicated so this test cannot drift from the real migrations.
func pgMigrationFiles() []string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred(), "reading migrations dir %s", dir)

	var files []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files
}

// setupPostgresStore resets dsn's public schema to empty and reapplies every
// migration, so each spec starts from a clean, known schema against a
// persistent dev Postgres rather than requiring a throwaway database per
// run. It calls Skip (not Fail) when dsn is unreachable, matching this
// repo's real-backend integration test convention (see
// internal/coachapi/queue/redisstream/redisstream_conformance_test.go):
// skip cleanly rather than fail or hang when the real backend isn't
// available. The returned pool is for direct SQL assertions (e.g. row
// counts after reclaim) that GetReport cannot exercise without false-green.
func setupPostgresStore(ctx context.Context, dsn string) (*postgres.Store, *pgxpool.Pool) {
	pool, err := pgxpool.New(ctx, dsn)
	Expect(err).NotTo(HaveOccurred(), "pgxpool.New")
	DeferCleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		Skip(fmt.Sprintf("could not connect to COACH_PG_DSN Postgres instance: %v", err))
	}

	_, err = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	Expect(err).NotTo(HaveOccurred(), "resetting public schema")

	for _, path := range pgMigrationFiles() {
		sqlBytes, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred(), "reading migration %s", path)
		_, err = pool.Exec(ctx, string(sqlBytes))
		Expect(err).NotTo(HaveOccurred(), "applying migration %s", path)
	}

	return postgres.NewStore(pool), pool
}
