package coachapi_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// recordingJudgeGateway records every Judge call and optionally delays or
// fails after N successful judgments (budget / packing acceptance).
type recordingJudgeGateway struct {
	inner modelgateway.Gateway

	mu        sync.Mutex
	reqs      []modelgateway.JudgmentRequest
	callCount atomic.Int32

	// delay is applied before each successful Judge (slow fake gateway).
	delay time.Duration

	// failAfterN, when > 0, returns ErrBudgetExceeded-style unavailability after
	// N successful Judge calls (used with short judgment wall via real sleep+wall).
	// When 0, all calls delegate to inner.
	//
	// For pack-budget tests we instead use a short MaxWallTime + delay so the
	// agentloop surfaces agentloop.ErrBudgetExceeded mid-phase.
	failAfterN int
	failErr    error

	// blockAfterN, when > 0, blocks on ctx.Done() for call number > N (1-based)
	// after recording the request. Used to force wall expiry on a specific pack.
	blockAfterN int

	// fixedJudgment, when non-nil, is returned instead of inner for non-blocking calls.
	fixedJudgment json.RawMessage
}

var _ = Describe("repo_baseline_scan packed judgment (local-LLM)", func() {
	When("a multi-finding fixture has ≥12 hidden_mutation signals across ≥3 paths with one hot path ≥6", func() {
		It("issues strictly fewer hidden_mutation_contextualization tool calls than finding count (packed judgment)", func() {
			body_handlerBaselineJudgmentPackAcceptanceTest_issuesStrictlyFewerHiddenMutationContextualizati_54()
		})

		It("embeds span-window evidence in pack args rather than full file content by default", func() {
			body_handlerBaselineJudgmentPackAcceptanceTest_embedsSpanWindowEvidenceInPackArgsRatherThanFull_145()
		})
	})

	When("judgment wall budget is exceeded mid-phase after successful packs", func() {
		It("persists agent findings from completed packs, records a judgment budget diagnostic, keeps deterministic findings, and completes the job", func() {
			body_handlerBaselineJudgmentPackAcceptanceTest_persistsAgentFindingsFromCompletedPacksRecordsAJ_221()
		})

		It("records judgment_budget_exceeded when the wall expires during the sole pack's in-flight Judge", func() {
			body_handlerBaselineJudgmentPackAcceptanceTest_recordsJudgmentBudgetExceededWhenTheWallExpiresD_274()
		})

		It("counts judged= as source=agent findings, not diagnostics-only pack items", func() {
			body_handlerBaselineJudgmentPackAcceptanceTest_countsJudgedAsSourceAgentFindingsNotDiagnosticsO_321()
		})
	})

	When("the parent context is canceled during packed judgment", func() {
		It("still aborts without complete-as-success", func() {
			root := multiHiddenMutationFixtureRoot()
			ctx, cancel := context.WithCancel(context.Background())

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
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
			body_handlerBaselineJudgmentPackAcceptanceTest_appliesJudgmentMaxWallTimeDefault10mOnTheJudgmen_395()
		})
	})
})
