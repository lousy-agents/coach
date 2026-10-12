package baseline_test

import (
	"context"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectJudgmentLoopUsesJudgmentWallBudget() {
	root := multiHiddenMutationFixtureRoot()

	var observed []*agentloop.Loop
	h := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "pack-owner",
		SmokeRepoName:    "pack-repo",
		Gateway:          modelgateway.NewStubGateway(),
		// Explicit non-default to prove config wiring (zero → 10m covered below).
		JudgmentMaxWallTime: 7 * time.Minute,
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = append(observed, loop)
		},
	})

	w := newCaptureWriter()
	_, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "pack-owner",
		RepoName:  "pack-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())
	Expect(observed).NotTo(BeEmpty())

	// Judgment loop is the one that recorded hidden_mutation calls and
	// must carry JudgmentMaxWallTime (not the residual analyze wall).
	judgmentLoop := firstLoopCalling(observed, rubrics.IDHiddenMutationContextualization)
	Expect(judgmentLoop).NotTo(BeNil(), "judgment loop must be observed")
	Expect(judgmentLoop.Budget().MaxWallTime).To(Equal(7*time.Minute),
		"judgment loop wall must come from JudgmentMaxWallTime, not shared analyze residual")

	// Default (zero) → 10 minutes.
	var observedDefault []*agentloop.Loop
	hDefault := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "pack-owner",
		SmokeRepoName:    "pack-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ObserveLoop: func(loop *agentloop.Loop) {
			observedDefault = append(observedDefault, loop)
		},
	})
	_, err = hDefault(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "pack-owner",
		RepoName:  "pack-repo",
	}), newCaptureWriter())
	Expect(err).NotTo(HaveOccurred())
	jDefault := firstLoopCalling(observedDefault, rubrics.IDHiddenMutationContextualization)
	Expect(jDefault).NotTo(BeNil())
	Expect(jDefault.Budget().MaxWallTime).To(Equal(10*time.Minute),
		"zero JudgmentMaxWallTime must default to 10m")
}

func firstLoopCalling(loops []*agentloop.Loop, name string) *agentloop.Loop {
	for _, loop := range loops {
		if loopHasCall(loop, name) {
			return loop
		}
	}
	return nil
}

func loopHasCall(loop *agentloop.Loop, name string) bool {
	for _, c := range loop.Calls() {
		if c.Name == name {
			return true
		}
	}
	return false
}
