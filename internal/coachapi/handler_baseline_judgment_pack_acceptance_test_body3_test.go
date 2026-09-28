package coachapi_test

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineJudgmentPackAcceptanceTest_persistsAgentFindingsFromCompletedPacksRecordsAJ_221() {
	root := multiHiddenMutationFixtureRoot()
	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())

	recGW.delay = 80 * time.Millisecond

	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath:    root,
		SmokeRepoOwner:      "pack-owner",
		SmokeRepoName:       "pack-repo",
		Gateway:             recGW,
		JudgmentMaxWallTime: 100 * time.Millisecond,

		PackConfig: rubrics.PackConfig{MaxFindingsPerJudgmentPack: 2},
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "pack-owner",
		RepoName:  "pack-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred(),
		"judgment budget exceed must complete the job (Story 2 / baseline Story 5 degrade)")
	Expect(completion).NotTo(BeNil())

	det, _, detHidden, agentHidden := countFindingsBySource(w.findings)
	Expect(det).To(BeNumerically(">=", 1))
	Expect(detHidden).To(BeNumerically(">=", 12),
		"deterministic hidden-mutation findings must remain complete")
	Expect(agentHidden).To(BeNumerically(">=", 1),
		"at least one pack's agent findings must already be persisted before budget exceed")
	Expect(agentHidden).To(BeNumerically("<", detHidden),
		"budget exceed mid-phase must leave some findings unjudged; agent=%d det=%d",
		agentHidden, detHidden)

	var sawBudgetDiag bool
	for _, d := range w.diagnostics {
		msg := strings.ToLower(d.Message)
		scope := strings.ToLower(d.Scope)
		if strings.Contains(msg, "judgment_budget_exceeded") ||
			(strings.Contains(msg, "judged=") && strings.Contains(msg, "remaining=")) ||
			(strings.Contains(scope, "judgment") && strings.Contains(scope, "budget")) {
			sawBudgetDiag = true

			if strings.Contains(d.Message, "judged=") {
				Expect(d.Message).To(ContainSubstring("remaining="))
			}
		}
	}
	Expect(sawBudgetDiag).To(BeTrue(),
		"must record judgment budget diagnostic with judged/remaining; got %#v", w.diagnostics)
}

func body_handlerBaselineJudgmentPackAcceptanceTest_recordsJudgmentBudgetExceededWhenTheWallExpiresD_274() {

	root := multiHiddenMutationFixtureRoot()
	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())
	recGW.delay = 200 * time.Millisecond

	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath:           root,
		SmokeRepoOwner:             "pack-owner",
		SmokeRepoName:              "pack-repo",
		Gateway:                    recGW,
		JudgmentMaxWallTime:        50 * time.Millisecond,
		MaxHiddenMutationJudgments: -1,

		PackConfig: rubrics.PackConfig{
			MaxFindingsPerJudgmentPack:      50,
			JudgmentFileAffinityMinFindings: 100,
		},
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "pack-owner",
		RepoName:  "pack-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())
	Expect(completion).NotTo(BeNil())

	_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
	Expect(detHidden).To(BeNumerically(">=", 12))
	Expect(agentHidden).To(Equal(0),
		"sole pack wall death mid-Judge must not invent agent rows")

	var budgetMsg string
	for _, d := range w.diagnostics {
		if strings.Contains(d.Message, "judgment_budget_exceeded") ||
			d.Scope == "judgment_budget" {
			budgetMsg = d.Message
			break
		}
	}
	Expect(budgetMsg).NotTo(BeEmpty(),
		"sole-pack wall expiry must emit judgment_budget_exceeded; got %#v", w.diagnostics)
	Expect(budgetMsg).To(ContainSubstring("judged=0"))
	Expect(budgetMsg).To(ContainSubstring("remaining="))
}

func body_handlerBaselineJudgmentPackAcceptanceTest_countsJudgedAsSourceAgentFindingsNotDiagnosticsO_321() {

	root := multiHiddenMutationFixtureRoot()
	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())
	recGW.fixedJudgment = json.RawMessage(`{"items":[]}`)
	recGW.blockAfterN = 1

	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath:    root,
		SmokeRepoOwner:      "pack-owner",
		SmokeRepoName:       "pack-repo",
		Gateway:             recGW,
		JudgmentMaxWallTime: 80 * time.Millisecond,
		PackConfig:          rubrics.PackConfig{MaxFindingsPerJudgmentPack: 4},
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "pack-owner",
		RepoName:  "pack-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())
	Expect(completion).NotTo(BeNil())

	_, _, _, agentHidden := countFindingsBySource(w.findings)
	Expect(agentHidden).To(Equal(0))

	var budgetMsg string
	for _, d := range w.diagnostics {
		if strings.Contains(d.Message, "judgment_budget_exceeded") ||
			d.Scope == "judgment_budget" {
			budgetMsg = d.Message
			break
		}
	}
	Expect(budgetMsg).NotTo(BeEmpty(),
		"expected judgment_budget_exceeded; got %#v", w.diagnostics)
	Expect(budgetMsg).To(ContainSubstring("judged=0"),
		"diagnostics-only pack must not inflate judged=; got %q", budgetMsg)
}
