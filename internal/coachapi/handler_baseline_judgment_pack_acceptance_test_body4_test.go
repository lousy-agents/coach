package coachapi_test

import (
	"context"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineJudgmentPackAcceptanceTest_appliesJudgmentMaxWallTimeDefault10mOnTheJudgmen_395() {
	root := multiHiddenMutationFixtureRoot()

	var observed []*agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "pack-owner",
		SmokeRepoName:    "pack-repo",
		Gateway:          modelgateway.NewStubGateway(),

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
	var judgmentLoop *agentloop.Loop
	for _, loop := range observed {
		for _, c := range loop.Calls() {
			if c.Name == rubrics.IDHiddenMutationContextualization {
				judgmentLoop = loop
				break
			}
		}
	}
	Expect(judgmentLoop).NotTo(BeNil(), "judgment loop must be observed")
	Expect(judgmentLoop.Budget().MaxWallTime).To(Equal(7*time.Minute),
		"judgment loop wall must come from JudgmentMaxWallTime, not shared analyze residual")

	// Default (zero) → 10 minutes.
	var observedDefault []*agentloop.Loop
	hDefault := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
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
	var jDefault *agentloop.Loop
	for _, loop := range observedDefault {
		for _, c := range loop.Calls() {
			if c.Name == rubrics.IDHiddenMutationContextualization {
				jDefault = loop
				break
			}
		}
	}
	Expect(jDefault).NotTo(BeNil())
	Expect(jDefault.Budget().MaxWallTime).To(Equal(10*time.Minute),
		"zero JudgmentMaxWallTime must default to 10m")
}
