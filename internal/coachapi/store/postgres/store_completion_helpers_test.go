package postgres_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/postgres"
)

// expectCompletionAssemblesMemoryStoreReportShape records one completion and
// requires GetReport to assemble the same report shape memory.Store
// produces for it.
func expectCompletionAssemblesMemoryStoreReportShape(ctx context.Context, store *postgres.Store) {
	job := pgQueuedJob("66666666-6666-6666-6666-666666666666")
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
				ID:          "66666666-0000-0000-0000-000000000001",
				JobID:       job.ID,
				Attempt:     1,
				Source:      coachapi.FindingSourceDeterministic,
				Payload:     json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"pkg/example/service.go"}`),
				PayloadHash: "hash-det-1",
			},
			{
				ID:            "66666666-0000-0000-0000-000000000002",
				JobID:         job.ID,
				Attempt:       1,
				Source:        coachapi.FindingSourceAgent,
				RubricID:      &agentRubricID,
				RubricVersion: &agentRubricVersion,
				ModelIdentity: &agentModelIdentity,
				Payload:       json.RawMessage(`{"judgment":"concern","rationale":"mutation hides input state that will surprise reviewers","confidence":"high","suggested_focus":"constructor parameter assignment"}`),
				PayloadHash:   "hash-agent-1",
			},
		},
		Diagnostics: []coachapi.JobDiagnostic{
			{
				ID:      "66666666-0000-0000-0000-000000000003",
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
	Expect(*gotJob.FinishedAt).To(BeTemporally("==", finishedAt))
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
				Payload:       json.RawMessage(`{"judgment":"concern","rationale":"mutation hides input state that will surprise reviewers","confidence":"high","suggested_focus":"constructor parameter assignment"}`),
			},
		},
		Diagnostics: []coachapi.Diagnostic{
			{Scope: "file:pkg/example/legacy.py", Message: "unsupported language"},
		},
		Error:       nil,
		Versions:    completion.Versions,
		GeneratedAt: generatedAt,
	}

	Expect(report.ReportVersion).To(Equal(wantReport.ReportVersion))
	Expect(report.JobID).To(Equal(wantReport.JobID))
	Expect(report.Kind).To(Equal(wantReport.Kind))
	Expect(report.Params).To(MatchJSON(wantReport.Params))
	Expect(report.CommitSHA).To(Equal(wantReport.CommitSHA))
	Expect(report.Summary).To(Equal(wantReport.Summary))
	Expect(report.Error).To(BeNil())
	Expect(report.Versions).To(Equal(wantReport.Versions))
	Expect(report.GeneratedAt).To(BeTemporally("==", wantReport.GeneratedAt))
	Expect(report.Diagnostics).To(Equal(wantReport.Diagnostics))

	Expect(report.Findings).To(HaveLen(len(wantReport.Findings)))
	for i, want := range wantReport.Findings {
		got := report.Findings[i]
		Expect(got.Source).To(Equal(want.Source), "Findings[%d].Source", i)
		Expect(got.RubricID).To(Equal(want.RubricID), "Findings[%d].RubricID", i)
		Expect(got.RubricVersion).To(Equal(want.RubricVersion), "Findings[%d].RubricVersion", i)
		Expect(got.ModelIdentity).To(Equal(want.ModelIdentity), "Findings[%d].ModelIdentity", i)
		Expect(got.Payload).To(MatchJSON(want.Payload), "Findings[%d].Payload", i)
	}
}
