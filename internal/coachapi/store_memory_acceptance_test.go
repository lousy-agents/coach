package coachapi_test

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
)

func newQueuedJob(id string) coachapi.Job {
	return coachapi.Job{
		ID:                id,
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets","ref":"main"}`),
		Status:            coachapi.JobStatusQueued,
		CreatedAt:         time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		Attempt:           0,
		CreatedByProvider: "github",
		CreatedBySubject:  "12345",
		CreatedByLogin:    "octocat",
	}
}

var _ = Describe("coachapi.MemoryStore", func() {
	var (
		ctx   context.Context
		store *coachapi.MemoryStore
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = coachapi.NewMemoryStore()
	})

	When("a job is created", func() {
		It("round-trips every field through GetJob", func() {
			job := newQueuedJob("11111111-1111-1111-1111-111111111111")

			Expect(store.CreateJob(ctx, job)).To(Succeed())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal(job.ID))
			Expect(got.Kind).To(Equal(job.Kind))
			Expect(got.Params).To(MatchJSON(job.Params))
			Expect(got.Status).To(Equal(coachapi.JobStatusQueued))
			Expect(got.CreatedAt).To(Equal(job.CreatedAt))
			Expect(got.Attempt).To(Equal(0))
			Expect(got.CreatedByProvider).To(Equal("github"))
			Expect(got.CreatedBySubject).To(Equal("12345"))
			Expect(got.CreatedByLogin).To(Equal("octocat"))
			Expect(got.Error).To(BeNil())
			Expect(got.StartedAt).To(BeNil())
			Expect(got.FinishedAt).To(BeNil())
		})

		It("does not let a caller mutate store state through the returned Job", func() {
			job := newQueuedJob("22222222-2222-2222-2222-222222222222")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			got, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			got.Params[0] = 'X'

			got2, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got2.Params).To(MatchJSON(job.Params), "returned Job must be a deep copy, not a view into store state")
		})

		It("rejects a second CreateJob for the same id", func() {
			job := newQueuedJob("66666666-6666-6666-6666-666666666666")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			err := store.CreateJob(ctx, job)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeFalse(), "a duplicate-id failure is a store failure, not a not-found")
		})
	})

	When("GetJob is called with an id that was never created", func() {
		It("returns an error satisfying errors.Is(err, coachapi.ErrJobNotFound)", func() {
			_, err := store.GetJob(ctx, "does-not-exist")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue())
		})
	})

	When("a job attempt completes successfully", func() {
		It("marks the job completed and assembles the Report from the recorded completion", func() {
			job := newQueuedJob("33333333-3333-3333-3333-333333333333")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			agentRubricID := "hidden_mutation_contextualization"
			agentRubricVersion := "1"
			agentModelIdentity := "stub-model@v1"
			generatedAt := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
			finishedAt := time.Date(2026, 1, 15, 11, 59, 0, 0, time.UTC)

			completion := coachapi.Completion{
				Attempt:   1,
				CommitSHA: "abc123def4567890abc123def4567890abc123de",
				Findings: []coachapi.JobFinding{
					{
						ID:          "finding-1",
						JobID:       job.ID,
						Attempt:     1,
						Source:      coachapi.FindingSourceDeterministic,
						Payload:     json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`),
						PayloadHash: "hash-det-1",
					},
					{
						ID:            "finding-2",
						JobID:         job.ID,
						Attempt:       1,
						Source:        coachapi.FindingSourceAgent,
						RubricID:      &agentRubricID,
						RubricVersion: &agentRubricVersion,
						ModelIdentity: &agentModelIdentity,
						Payload:       json.RawMessage(`{"judgment":"actionable","rule_id":"state.hidden_input_mutation"}`),
						PayloadHash:   "hash-agent-1",
					},
				},
				Diagnostics: []coachapi.JobDiagnostic{
					{
						ID:      "diag-1",
						JobID:   job.ID,
						Attempt: 1,
						Scope:   "file:pkg/example/legacy.py",
						Message: "unsupported language",
					},
				},
				Versions: coachapi.ReportVersions{
					Analyzer: "codesignal@1",
					Rubrics: map[string]string{
						"change_cohesion":                   "1",
						"hidden_mutation_contextualization": "1",
					},
				},
				FinishedAt:  finishedAt,
				GeneratedAt: generatedAt,
			}

			Expect(store.RecordCompletion(ctx, job.ID, completion)).To(Succeed())

			gotJob, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotJob.Status).To(Equal(coachapi.JobStatusCompleted))
			Expect(gotJob.Attempt).To(Equal(1))
			Expect(gotJob.FinishedAt).NotTo(BeNil())
			Expect(*gotJob.FinishedAt).To(Equal(finishedAt))
			Expect(gotJob.Error).To(BeNil())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())

			wantReport := coachapi.Report{
				ReportVersion: coachapi.ReportVersion1,
				JobID:         job.ID,
				Kind:          coachapi.JobKindRepoBaselineScan,
				Params:        job.Params,
				CommitSHA:     completion.CommitSHA,
				Summary: coachapi.ReportSummary{
					FindingCounts: map[string]map[string]int{
						"deterministic": {"state.hidden_input_mutation": 1},
						"agent":         {"hidden_mutation_contextualization": 1},
					},
				},
				Findings: []coachapi.Finding{
					{
						Source:  coachapi.FindingSourceDeterministic,
						Payload: json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`),
					},
					{
						Source:        coachapi.FindingSourceAgent,
						RubricID:      &agentRubricID,
						RubricVersion: &agentRubricVersion,
						ModelIdentity: &agentModelIdentity,
						Payload:       json.RawMessage(`{"judgment":"actionable","rule_id":"state.hidden_input_mutation"}`),
					},
				},
				Diagnostics: []coachapi.Diagnostic{
					{Scope: "file:pkg/example/legacy.py", Message: "unsupported language"},
				},
				Error:       nil,
				Versions:    completion.Versions,
				GeneratedAt: generatedAt,
			}

			Expect(report).To(Equal(wantReport))

			raw, err := json.Marshal(report)
			Expect(err).NotTo(HaveOccurred())
			var asMap map[string]json.RawMessage
			Expect(json.Unmarshal(raw, &asMap)).To(Succeed())
			Expect(asMap).To(HaveKey("report_version"))
			Expect(string(asMap["error"])).To(Equal("null"))
		})
	})

	When("GetReport is called for a job that has never completed", func() {
		It("returns an error satisfying errors.Is(err, coachapi.ErrJobNotFound)", func() {
			job := newQueuedJob("55555555-5555-5555-5555-555555555555")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			_, err := store.GetReport(ctx, job.ID)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue())
		})
	})

	When("a job attempt fails", func() {
		It("marks the job failed with the recorded error message and finish time", func() {
			job := newQueuedJob("44444444-4444-4444-4444-444444444444")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			finishedAt := time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC)
			Expect(store.RecordFailure(ctx, job.ID, "clone failed: timeout", finishedAt)).To(Succeed())

			gotJob, err := store.GetJob(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotJob.Status).To(Equal(coachapi.JobStatusFailed))
			Expect(gotJob.Error).NotTo(BeNil())
			Expect(*gotJob.Error).To(Equal("clone failed: timeout"))
			Expect(gotJob.FinishedAt).NotTo(BeNil())
			Expect(*gotJob.FinishedAt).To(Equal(finishedAt))
		})
	})

	When("RecordCompletion or RecordFailure target an id that was never created", func() {
		It("returns an error satisfying errors.Is(err, coachapi.ErrJobNotFound)", func() {
			err := store.RecordCompletion(ctx, "does-not-exist", coachapi.Completion{})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue())

			err = store.RecordFailure(ctx, "does-not-exist", "boom", time.Now())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, coachapi.ErrJobNotFound)).To(BeTrue())
		})
	})

	When("CreateJob and GetJob are called concurrently for distinct jobs", func() {
		It("does not race", func() {
			const n = 20
			var wg sync.WaitGroup
			errs := make(chan error, 2*n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					id := fmt.Sprintf("job-%d", i)
					job := newQueuedJob(id)
					errs <- store.CreateJob(ctx, job)
					_, err := store.GetJob(ctx, id)
					errs <- err
				}(i)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	// Reviewer finding #2: MemoryStore must stamp lease jobID/attempt.
	When("InsertFindings is given findings stamped with a wrong Attempt", func() {
		It("persists the fenced lease attempt so GetReport includes them", func() {
			job := newQueuedJob("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			start := time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC)
			lease, err := store.ClaimJob(ctx, job.ID, "worker-a", start, 60*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease.Attempt).To(Equal(1))

			Expect(store.InsertFindings(ctx, job.ID, "worker-a", lease.Attempt, []coachapi.JobFinding{{
				ID:          "finding-wrong-attempt",
				JobID:       "other-job",
				Attempt:     0,
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"stamped"}`),
				PayloadHash: "hash-stamped",
				CreatedAt:   start,
			}})).To(Succeed())

			Expect(store.CompleteJob(ctx, job.ID, "worker-a", lease.Attempt, coachapi.Completion{
				Attempt:     lease.Attempt,
				CommitSHA:   "abc123def4567890abc123def4567890abc123de",
				Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt:  start,
				GeneratedAt: start,
			})).To(Succeed())

			report, err := store.GetReport(ctx, job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Findings).To(HaveLen(1), "findings must be stored under the lease attempt, not client Attempt")
			Expect(string(report.Findings[0].Payload)).To(ContainSubstring(`"rule_id":"stamped"`))
		})
	})

	// Reviewer finding #3 contract: age == staleAfter is reclaimable.
	When("a running job's heartbeat age equals staleAfter exactly", func() {
		It("allows ClaimJob to reclaim and ReleaseStaleRunning to release", func() {
			job := newQueuedJob("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
			Expect(store.CreateJob(ctx, job)).To(Succeed())

			const staleAfter = 60 * time.Second
			start := time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC)
			_, err := store.ClaimJob(ctx, job.ID, "worker-a", start, staleAfter)
			Expect(err).NotTo(HaveOccurred())

			boundary := start.Add(staleAfter)
			lease2, err := store.ClaimJob(ctx, job.ID, "worker-b", boundary, staleAfter)
			Expect(err).NotTo(HaveOccurred())
			Expect(lease2.Attempt).To(Equal(2))

			job2 := newQueuedJob("cccccccc-cccc-cccc-cccc-cccccccccccc")
			Expect(store.CreateJob(ctx, job2)).To(Succeed())
			_, err = store.ClaimJob(ctx, job2.ID, "worker-a", start, staleAfter)
			Expect(err).NotTo(HaveOccurred())

			released, err := store.ReleaseStaleRunning(ctx, boundary, staleAfter)
			Expect(err).NotTo(HaveOccurred())
			Expect(released).To(BeNumerically(">=", 1))

			got, err := store.GetJob(ctx, job2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Status).To(Equal(coachapi.JobStatusQueued))
		})
	})
})
