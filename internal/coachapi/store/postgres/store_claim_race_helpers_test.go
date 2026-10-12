package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/postgres"
)

// expectExactlyOneConcurrentClaimWins races 20 ClaimJob calls on one queued
// job and requires exactly one lease, every loser seeing ErrNotClaimable.
func expectExactlyOneConcurrentClaimWins(ctx context.Context, store *postgres.Store) {
	job := pgQueuedJob("dddddddd-dddd-dddd-dddd-dddddddddddd")
	Expect(store.CreateJob(ctx, job)).To(Succeed())

	const n = 20
	start := time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)
	results := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.ClaimJob(ctx, job.ID, fmt.Sprintf("pg-w-%d", i), start, 60*time.Second)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	var wins, losses int
	for err := range results {
		if err == nil {
			wins++
			continue
		}
		Expect(errors.Is(err, coachapi.ErrNotClaimable)).To(BeTrue(), "unexpected claim err: %v", err)
		losses++
	}
	Expect(wins).To(Equal(1), "exactly one Postgres claim must succeed")
	Expect(losses).To(Equal(n - 1))

	got, err := store.GetJob(ctx, job.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(got.Status).To(Equal(coachapi.JobStatusRunning))
	Expect(got.Attempt).To(Equal(1))
	Expect(got.ClaimedBy).NotTo(BeNil())
}

// expectZombieInsertLosesToReclaim holds worker A's fenced InsertFindings
// transaction open while worker B reclaims the job, then requires A's insert
// to fail with ErrClaimLost and leave nothing in B's report.
func expectZombieInsertLosesToReclaim(ctx context.Context, store *postgres.Store) {
	job := pgQueuedJob("99999999-9999-9999-9999-999999999999")
	Expect(store.CreateJob(ctx, job)).To(Succeed())

	start := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	lease1, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
	Expect(err).NotTo(HaveOccurred())
	Expect(lease1.Attempt).To(Equal(1))

	entered := make(chan struct{})
	release := make(chan struct{})
	postgres.SetFenceHoldForTest(func() {
		close(entered)
		<-release
	})
	DeferCleanup(func() { postgres.SetFenceHoldForTest(nil) })

	var insertErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		insertErr = store.InsertFindings(ctx, job.ID, "worker-a", 1, []coachapi.JobFinding{{
			ID:          "99999999-0000-0000-0000-000000000001",
			JobID:       job.ID,
			Attempt:     1,
			Source:      coachapi.FindingSourceDeterministic,
			Payload:     json.RawMessage(`{"rule_id":"zombie"}`),
			PayloadHash: "hash-zombie",
			CreatedAt:   start,
		}})
	}()

	Eventually(entered).Should(BeClosed())

	reclaimAt := start.Add(61 * time.Second)
	lease2, err := store.ClaimJob(ctx, job.ID, "worker-b", reclaimAt, 60*time.Second)
	Expect(err).NotTo(HaveOccurred(), "reclaim must commit while the zombie insert tx is open past its fence check")
	Expect(lease2.Attempt).To(Equal(2))

	close(release)
	wg.Wait()

	Expect(errors.Is(insertErr, coachapi.ErrClaimLost)).To(BeTrue(), "zombie InsertFindings err = %v", insertErr)

	// Completing as B with no findings must yield an empty report —
	// the zombie row must not have been committed.
	Expect(store.CompleteJob(ctx, job.ID, "worker-b", 2, coachapi.Completion{
		Attempt:     2,
		CommitSHA:   "abc123def4567890abc123def4567890abc123de",
		Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
		FinishedAt:  reclaimAt,
		GeneratedAt: reclaimAt,
	})).To(Succeed())
	report, err := store.GetReport(ctx, job.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(report.Findings).To(BeEmpty(), "zombie findings must not pollute the reclaimed attempt's report")
}
