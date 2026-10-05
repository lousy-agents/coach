package baseline_test

import (
	"context"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectSmokeBaselineAssemblesReport() {
	h := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway:          modelgateway.NewStubGateway(),
	})
	job := baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
		Ref:       "main",
	})
	store, w := newMemoryFencedWriter(job)
	completion, err := h(context.Background(), job, w)
	Expect(err).NotTo(HaveOccurred())
	Expect(completion).NotTo(BeNil())

	lease := w.Lease()
	Expect(store.CompleteJob(context.Background(), lease.JobID, lease.WorkerID, lease.Attempt, *completion)).To(Succeed())

	report, err := store.GetReport(context.Background(), job.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(report.CommitSHA).To(Equal("local-fixture"))
	Expect(report.Versions.Analyzer).NotTo(BeEmpty())
	Expect(report.Versions.Rubrics).NotTo(BeEmpty())
	Expect(report.Versions.Rubrics).To(HaveKey(rubrics.IDHiddenMutationContextualization))
	Expect(report.Versions.Rubrics).To(HaveKey(rubrics.IDChangeCohesion))
	Expect(report.Findings).NotTo(BeEmpty())
	var sawDet, sawAgent bool
	for _, f := range report.Findings {
		switch f.Source {
		case coachapi.FindingSourceDeterministic:
			sawDet = true
			Expect(f.RubricID).To(BeNil())
		case coachapi.FindingSourceAgent:
			sawAgent = true
			Expect(f.RubricID).NotTo(BeNil())
		}
	}
	Expect(sawDet).To(BeTrue(), "report must include source=deterministic findings")
	Expect(sawAgent).To(BeTrue(), "report must include source=agent findings from stub judgments")
	Expect(report.Error).To(BeNil())
	Expect(report.ReportVersion).To(Equal(coachapi.ReportVersion1))
}
