package baseline_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

var _ = Describe("repo_baseline_scan job handler", func() {
	When("client-supplied clone URLs appear in stored job params", func() {
		// API DisallowUnknownFields is covered in httpapi/server_acceptance_test; this
		// owns the handler params parser permanent reject.
		It("rejects git_url via the handler production params parser", func() {
			raw := []byte(`{"repo_owner":"acme","repo_name":"widgets","git_url":"https://evil.example/x.git"}`)
			h := baseline.NewScanHandler(baseline.ScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "acme",
				SmokeRepoName:    "widgets",
				Gateway:          modelgateway.NewStubGateway(),
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{RepoOwner: "acme", RepoName: "widgets"})
			job.Params = raw
			completion, err := h(context.Background(), job, newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(And(
				ContainSubstring("git_url"),
				ContainSubstring("not allowed"),
			), "handler must reject sneaked git_url via production parser; got %v", err)
		})

		It("rejects clone_url via the handler production params parser", func() {
			raw := []byte(`{"repo_owner":"acme","repo_name":"widgets","clone_url":"https://evil.example/x.git"}`)
			h := baseline.NewScanHandler(baseline.ScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "acme",
				SmokeRepoName:    "widgets",
				Gateway:          modelgateway.NewStubGateway(),
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{RepoOwner: "acme", RepoName: "widgets"})
			job.Params = raw
			completion, err := h(context.Background(), job, newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(And(
				ContainSubstring("clone_url"),
				ContainSubstring("not allowed"),
			), "handler must reject sneaked clone_url via production parser; got %v", err)
		})
	})
})
