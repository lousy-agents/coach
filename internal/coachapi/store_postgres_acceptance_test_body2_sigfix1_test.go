package coachapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type sigbodystorePostgresAcceptanceTestcoachapiPostgresStore96617 struct {
	ctx   *context.Context
	store **coachapi.PostgresStore
}

func (sigRecv *sigbodystorePostgresAcceptanceTestcoachapiPostgresStore96617) call() {
	ctx := *sigRecv.ctx
	store := *sigRecv.store
	job := pgQueuedJob("99999999-9999-9999-9999-999999999999")
	Expect(store.CreateJob(ctx, job)).To(Succeed())

	start := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	lease1, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
	Expect(err).NotTo(HaveOccurred())
	Expect(lease1.Attempt).To(Equal(1))

	entered := make(chan struct{})
	release := make(chan struct{})
	coachapi.SetFenceHoldForTest(func() {
		close(entered)
		<-release
	})
	DeferCleanup(func() { coachapi.SetFenceHoldForTest(nil) })

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
