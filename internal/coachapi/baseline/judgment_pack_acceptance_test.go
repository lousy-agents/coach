package baseline_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

var _ = Describe("repo_baseline_scan packed judgment (local-LLM)", func() {
	When("a multi-finding fixture has ≥12 hidden_mutation signals across ≥3 paths with one hot path ≥6", func() {
		It("issues strictly fewer hidden_mutation_contextualization tool calls than finding count (packed judgment)", func() {
			expectPackedJudgmentIssuesFewerCallsThanFindings()
		})

		It("embeds span-window evidence in pack args rather than full file content by default", func() {
			expectPackArgsEmbedSpanWindowsNotFullFiles()
		})
	})

	When("judgment wall budget is exceeded mid-phase after successful packs", func() {
		It("persists agent findings from completed packs, records a judgment budget diagnostic, keeps deterministic findings, and completes the job", func() {
			expectBudgetStopKeepsCompletedPackFindings()
		})

		It("records judgment_budget_exceeded when the wall expires during the sole pack's in-flight Judge", func() {
			expectSolePackWallExpiryRecordsBudgetExceeded()
		})

		It("counts judged= as source=agent findings, not diagnostics-only pack items", func() {
			expectJudgedCountsOnlyAgentFindings()
		})
	})

	When("the parent context is canceled during packed judgment", func() {
		It("still aborts without complete-as-success", func() {
			root := multiHiddenMutationFixtureRoot()
			ctx, cancel := context.WithCancel(context.Background())

			h := baseline.NewScanHandler(baseline.ScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "pack-owner",
				SmokeRepoName:    "pack-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ConfigureLoop: func(loop *agentloop.Loop) {
					Expect(loop.Register(agentloop.ToolSpec{
						Name: rubrics.IDHiddenMutationContextualization,
						Handler: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
							cancel()
							return nil, context.Canceled
						},
					})).To(Succeed())
				},
			})

			completion, err := h(ctx, baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "pack-owner",
				RepoName:  "pack-repo",
			}), newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	When("judgment uses a separate wall budget from analyze", func() {
		It("applies JudgmentMaxWallTime (default 10m) on the judgment loop so analyze time does not alone exhaust judgment budget", func() {
			expectJudgmentLoopUsesJudgmentWallBudget()
		})
	})
})
