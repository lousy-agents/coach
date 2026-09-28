package coachapi_test

import (
	"context"

	"encoding/json"

	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_handlerBaselineAcceptanceTest_completesDeterministicFindingsWithoutAgentloopMa_728() {

	root := GinkgoT().TempDir()
	const n = 55
	for i := 0; i < n; i++ {
		name := filepath.Join(root, fmt.Sprintf("f%02d.go", i))
		content := fmt.Sprintf("package p%d\n\nfunc F%d() {}\n", i, i)
		Expect(os.WriteFile(name, []byte(content), 0o644)).To(Succeed())
	}

	signalFile := filepath.Join(root, "mutate.go")
	Expect(os.WriteFile(signalFile, []byte(`package p

type C struct{ N string }

func Mut(c *C, n string) { c.N = n }
`), 0o644)).To(Succeed())

	var observed []*agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "big-owner",
		SmokeRepoName:    "big-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ObserveLoop:      func(loop *agentloop.Loop) { observed = append(observed, loop) },
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "big-owner",
		RepoName:  "big-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred(),
		"repos with >50 supported files must not fail with ErrBudgetExceeded/max_tool_calls")
	Expect(completion).NotTo(BeNil())
	Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeFalse())

	var det int
	for _, f := range w.findings {
		if f.Source == coachapi.FindingSourceDeterministic {
			det++
		}
	}
	Expect(det).To(BeNumerically(">=", 1),
		"deterministic codesignal findings must be produced for the large tree")

	Expect(observed).NotTo(BeEmpty())
	var analyzeLoop *agentloop.Loop
	analyzeCalls := 0
	for _, loop := range observed {
		(&sigbodyhandlerBaselineAcceptanceTestcompletesDeterministicFi{analyzeCalls: &analyzeCalls, analyzeLoop: &analyzeLoop, loop: loop}).call()

	}
	Expect(analyzeLoop).NotTo(BeNil())
	Expect(analyzeLoop.Budget().MaxToolCalls).To(BeNumerically(">", agentloop.DefaultMaxToolCalls),
		"baseline analyze loop budget must scale above DefaultMaxToolCalls for large trees")
	Expect(analyzeCalls).To(BeNumerically(">=", n),
		"handler must drive semantics_analyze for each supported file")
}

func body_handlerBaselineAcceptanceTest_stillCompletesWithDeterministicFindingsAlreadyWr_850() {
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: baselineFixtureRoot(),
		SmokeRepoOwner:   "smoke-owner",
		SmokeRepoName:    "smoke-repo",
		Gateway:          modelgateway.NewStubGateway(),
		ConfigureLoop: func(loop *agentloop.Loop) {

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
