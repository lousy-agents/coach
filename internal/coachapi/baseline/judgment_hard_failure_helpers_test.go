package baseline_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectHardJudgmentFailureKeepsWrittenFindings() {
	h := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ConfigureLoop: func(loop *agentloop.Loop) {
			// Plain hard failure (not gateway-unavailable degrade envelope).
			hard := func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
				return nil, errors.New("injected hard judgment failure")
			}
			Expect(loop.Register(agentloop.ToolSpec{
				Name:    rubrics.IDHiddenMutationContextualization,
				Handler: hard,
			})).To(Succeed())
			Expect(loop.Register(agentloop.ToolSpec{
				Name:    rubrics.IDChangeCohesion,
				Handler: hard,
			})).To(Succeed())
		},
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "smoke-owner",
		RepoName:  "smoke-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred(),
		"hard non-cancel judgment error must not FailJob after deterministic analysis (Story 5)")
	Expect(completion).NotTo(BeNil())

	var det int
	for _, f := range w.findings {
		if f.Source == coachapi.FindingSourceDeterministic {
			det++
		}
		Expect(f.Source).NotTo(Equal(coachapi.FindingSourceAgent),
			"hard judgment failure must not leave partial source=agent findings")
	}
	Expect(det).To(BeNumerically(">=", 1),
		"deterministic findings must already be InsertFindings'd before judgment hard-fails")
	Expect(w.diagnostics).NotTo(BeEmpty(), "hard judgment failure must record a JobDiagnostic")
	var sawJudgmentDiag bool
	for _, d := range w.diagnostics {
		if strings.Contains(d.Message, "judgment") || strings.Contains(d.Scope, "judgment") ||
			strings.Contains(d.Message, "injected hard") || strings.Contains(d.Scope, "rubric") {
			sawJudgmentDiag = true
		}
	}
	Expect(sawJudgmentDiag).To(BeTrue(), "diagnostic should describe the judgment-phase failure")
}
