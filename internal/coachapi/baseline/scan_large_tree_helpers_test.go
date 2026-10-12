package baseline_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

func expectLargeTreeCompletesWithinToolCallBudget() {

	root := GinkgoT().TempDir()
	const n = 55
	for i := 0; i < n; i++ {
		name := filepath.Join(root, fmt.Sprintf("f%02d.go", i))
		content := fmt.Sprintf("package p%d\n\nfunc F%d() {}\n", i, i)
		Expect(os.WriteFile(name, []byte(content), 0o644)).To(Succeed())
	}
	// Signal-bearing file so we assert findings, not only nil error.
	signalFile := filepath.Join(root, "mutate.go")
	Expect(os.WriteFile(signalFile, []byte(`package p

type C struct{ N string }

func Mut(c *C, n string) { c.N = n }
`), 0o644)).To(Succeed())

	var observed []*agentloop.Loop
	h := baseline.NewScanHandler(baseline.ScanConfig{
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
		if calls := handlerAnalyzeCallCount(loop); calls > 0 {
			analyzeCalls += calls
			analyzeLoop = loop
		}
	}
	Expect(analyzeLoop).NotTo(BeNil())
	Expect(analyzeLoop.Budget().MaxToolCalls).To(BeNumerically(">", agentloop.DefaultMaxToolCalls),
		"baseline analyze loop budget must scale above DefaultMaxToolCalls for large trees")
	Expect(analyzeCalls).To(BeNumerically(">=", n),
		"handler must drive semantics_analyze for each supported file")
}

// handlerAnalyzeCallCount counts the handler-sourced semantics_analyze
// calls one loop recorded.
func handlerAnalyzeCallCount(loop *agentloop.Loop) int {
	n := 0
	for _, c := range loop.Calls() {
		if c.Source == agentloop.CallSourceHandler && c.Name == agentloop.ToolSemanticsAnalyze {
			n++
		}
	}
	return n
}
