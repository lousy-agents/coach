package baseline_test

import (
	"context"
	"sort"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectPackedCappedJudgmentFinishesUnderShortWall() {
	root := lousyIAMStyleHiddenMutationFixtureRoot()

	// Timing contract (false-green guard):
	//   wall 700ms, sleep 100ms/Judge → at most ~7 sequential 1:1
	//   gateway calls before wall. Pure 1:1 for 42 findings needs ~4.2s
	//   and cannot reach the 16-finding cap; with pack size 4 + cap 16,
	//   the path needs ~4 HM packs (~400ms) and yields many agent rows per
	//   call. The larger absolute window leaves room for scheduler and
	//   race-detector overhead while preserving the one-to-one guard.
	//   Assert agentHidden > max 1:1 calls under the wall so a revert to
	//   uncapped 1:1 fails this spec (1:1 yields ≤~7 agent rows here).
	const (
		judgeDelay   = 100 * time.Millisecond
		judgmentWall = 700 * time.Millisecond
		// Floor above what sequential 1:1 can produce under wall/delay.
		minAgentHiddenBeyondOneToOne = 8
	)

	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())
	recGW.delay = judgeDelay

	h := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "ampl-owner",
		SmokeRepoName:    "ampl-repo",
		Gateway:          recGW,
		// Explicit knobs from the local-LLM judgment spec.
		JudgmentMaxWallTime:        judgmentWall,
		MaxHiddenMutationJudgments: baseline.DefaultMaxHiddenMutationJudgments, // 16
		// Zero PackConfig → default pack size 4 (ApplyPackConfigDefaults).
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "ampl-owner",
		RepoName:  "ampl-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred(),
		"packed+capped path must complete the job under the short judgment wall")
	Expect(completion).NotTo(BeNil())

	_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
	Expect(detHidden).To(Equal(42),
		"fixture must yield 22+7+6+4+3 deterministic hidden_input_mutation signals; got %d", detHidden)

	pathCounts := deterministicHiddenMutationPathCounts(w.findings)
	Expect(len(pathCounts)).To(Equal(5), "expected 5 paths; got %v", pathCounts)
	gotCounts := make([]int, 0, len(pathCounts))
	for _, c := range pathCounts {
		gotCounts = append(gotCounts, c)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(gotCounts)))
	Expect(gotCounts).To(Equal(lousyIAMStyleHiddenMutationCounts),
		"path distribution must match lousy-iam style 22+7+6+4+3; paths=%v", pathCounts)

	Expect(agentHidden).To(BeNumerically(">=", 1),
		"packed path must persist ≥1 source=agent hidden_mutation finding under short wall")
	// False-green: pure 1:1 under the same wall can finish at most
	// ~wall/delay Judge calls (≈7), so agentHidden would stay ≤ that
	// bound. Packed packs (size 4) with cap 16 amplify findings/call.
	Expect(agentHidden).To(BeNumerically(">=", minAgentHiddenBeyondOneToOne),
		"agent rows must exceed pure 1:1 capacity under wall=%s delay=%s (1:1≤~7); agent=%d paths=%v",
		judgmentWall, judgeDelay, agentHidden, pathCounts)

	// Packing evidence: gateway Judge calls for hidden_mutation must be
	// far below the 42-signal finding count (and below the 16-cap if 1:1).
	hmJudge := 0
	for _, req := range recGW.requests() {
		if req.RubricID == rubrics.IDHiddenMutationContextualization {
			hmJudge++
		}
	}
	Expect(hmJudge).To(BeNumerically(">=", 1))
	Expect(hmJudge).To(BeNumerically("<", baseline.DefaultMaxHiddenMutationJudgments),
		"packed judgment must issue fewer HM Judge calls than the cap; judges=%d", hmJudge)
	Expect(hmJudge).To(BeNumerically("<", detHidden),
		"packed judgment must issue fewer HM Judge calls than findings; judges=%d findings=%d",
		hmJudge, detHidden)
}
