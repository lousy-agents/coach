package coachapi_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

var _ = Describe("repo_baseline_scan judgment priority cap (local-LLM Story 3)", func() {
	When("deterministic hidden_mutation count exceeds max_hidden_mutation_judgments", func() {
		It("judges a round-robin prioritized subset across paths, not the entire cap from the hot path, and records selected/omitted diagnostic", func() {
			body_handlerBaselineJudgmentCapAcceptanceTest_judgesARoundRobinPrioritizedSubsetAcrossPathsNot_22()
		})
	})

	When("deterministic hidden_mutation count is under the judgment cap", func() {
		It("judges all hidden_mutation signals and does not record a judgment cap diagnostic", func() {
			root := multiHiddenMutationFixtureRoot()

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "cap-owner",
				SmokeRepoName:    "cap-repo",
				Gateway:          modelgateway.NewStubGateway(),

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

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
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

// hotColdHiddenMutationFixtureRoot builds a temp tree with one hot path and
// several cold paths (20 + 3 + 3 + 2) for Story 3 priority-cap round-robin.
func hotColdHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()

	writeMutators := func(pkg, path string, n int) {
		body_handlerBaselineJudgmentCapAcceptanceTest_176(pkg, path, n, root)
	}

	writeMutators("hot", "hot.go", 20)
	writeMutators("colda", "cold_a.go", 3)
	writeMutators("coldb", "cold_b.go", 3)
	writeMutators("coldc", "cold_c.go", 2)
	return root
}

func hasJudgmentCapDiagnostic(diags []coachapi.JobDiagnostic) (found bool, message string) {
	for _, d := range diags {
		msg := d.Message
		scope := strings.ToLower(d.Scope)
		if strings.Contains(msg, "judgment_cap_omitted") ||
			scope == "judgment_cap" ||
			(strings.Contains(msg, "selected=") && strings.Contains(msg, "omitted=")) {
			return true, d.Message
		}
	}
	return false, ""
}
