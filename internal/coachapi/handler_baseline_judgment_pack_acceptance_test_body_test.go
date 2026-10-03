package coachapi_test

import (
	"context"
	"encoding/json"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineJudgmentPackAcceptanceTest_issuesStrictlyFewerHiddenMutationContextualizati_54() {
	root := multiHiddenMutationFixtureRoot()
	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())

	var observed []*agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "pack-owner",
		SmokeRepoName:    "pack-repo",
		Gateway:          recGW,
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = append(observed, loop)
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
	Expect(detHidden).To(BeNumerically(">=", 12),
		"fixture must yield ≥12 deterministic hidden_input_mutation signals; got %d", detHidden)
	Expect(agentHidden).To(Equal(detHidden),
		"one source=agent row per judged hidden-mutation signal")

	paths := map[string]int{}
	for _, f := range w.findings {
		if f.Source != coachapi.FindingSourceDeterministic {
			continue
		}
		if !strings.Contains(string(f.Payload), "hidden_input_mutation") {
			continue
		}
		var sig struct {
			Path string `json:"path"`
		}
		Expect(json.Unmarshal(f.Payload, &sig)).To(Succeed())
		paths[sig.Path]++
	}
	Expect(len(paths)).To(BeNumerically(">=", 3), "fixture paths: %v", paths)
	var maxPath int
	for _, c := range paths {
		(&sigbodyhandlerBaselineJudgmentPackAcceptanceTestissuesStrict{c: c, maxPath: &maxPath}).call()

	}
	Expect(maxPath).To(BeNumerically(">=", 6), "hot path finding count; paths=%v", paths)

	hmCalls := countHiddenMutationToolCalls(observed)
	Expect(hmCalls).To(BeNumerically(">=", 1))
	Expect(hmCalls).To(BeNumerically("<", detHidden),
		"packed judgment must issue fewer hidden_mutation tool calls than findings; calls=%d findings=%d",
		hmCalls, detHidden)

	// At least one multi-finding pack: a Judge request with batch OutputSchema
	// and ≥2 finding_ref mentions, or a tool call args items len ≥ 2.
	var sawMultiPackArgs bool
	for _, loop := range observed {
		if loop == nil {
			continue
		}
		(&sigbodyhandlerBaselineJudgmentPackAcceptanceTestissuesStrict0{loop: loop, sawMultiPackArgs: &sawMultiPackArgs}).call()

	}
	Expect(sawMultiPackArgs).To(BeTrue(),
		"at least one multi-finding pack tool call is required")

	hmJudge := 0
	for _, req := range recGW.requests() {
		if req.RubricID == rubrics.IDHiddenMutationContextualization {
			hmJudge++
		}
	}
	Expect(hmJudge).To(BeNumerically("<", detHidden),
		"gateway Judge calls for hidden_mutation must be packed; judges=%d findings=%d",
		hmJudge, detHidden)
}
