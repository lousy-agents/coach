package baseline_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

var _ = Describe("repo_baseline_scan judgment priority cap (local-LLM Story 3)", func() {
	When("deterministic hidden_mutation count exceeds max_hidden_mutation_judgments", func() {
		It("judges a round-robin prioritized subset across paths, not the entire cap from the hot path, and records selected/omitted diagnostic", func() {
			expectRoundRobinJudgmentCapAcrossPaths()
		})
	})

	When("deterministic hidden_mutation count is under the judgment cap", func() {
		It("judges all hidden_mutation signals and does not record a judgment cap diagnostic", func() {
			root := multiHiddenMutationFixtureRoot()

			h := baseline.NewScanHandler(baseline.ScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "cap-owner",
				SmokeRepoName:    "cap-repo",
				Gateway:          modelgateway.NewStubGateway(),
				// Zero → default 16; fixture has 14.
				MaxHiddenMutationJudgments: 0,
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "cap-owner",
				RepoName:  "cap-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(detHidden).To(BeNumerically(">=", 12))
			Expect(detHidden).To(BeNumerically("<=", 16),
				"under-cap fixture must stay at or below default cap; got %d", detHidden)
			Expect(agentHidden).To(Equal(detHidden),
				"when under cap every hidden_mutation signal is judged")

			found, msg := hasJudgmentCapDiagnostic(w.diagnostics)
			Expect(found).To(BeFalse(),
				"no cap diagnostic when nothing omitted; got %q in %#v", msg, w.diagnostics)
		})
	})

	When("MaxHiddenMutationJudgments is negative (unlimited)", func() {
		It("judges all hidden_mutation signals even when count exceeds the default 16", func() {
			root := hotColdHiddenMutationFixtureRoot()

			h := baseline.NewScanHandler(baseline.ScanConfig{
				SmokeFixturePath:           root,
				SmokeRepoOwner:             "cap-owner",
				SmokeRepoName:              "cap-repo",
				Gateway:                    modelgateway.NewStubGateway(),
				MaxHiddenMutationJudgments: -1,
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "cap-owner",
				RepoName:  "cap-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(detHidden).To(Equal(28))
			Expect(agentHidden).To(Equal(28),
				"negative max means unlimited; all findings judged")

			found, _ := hasJudgmentCapDiagnostic(w.diagnostics)
			Expect(found).To(BeFalse(), "unlimited must not emit cap diagnostic")
		})
	})
})
