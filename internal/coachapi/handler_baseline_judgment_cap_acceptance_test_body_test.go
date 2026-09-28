package coachapi_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

func body_handlerBaselineJudgmentCapAcceptanceTest_judgesARoundRobinPrioritizedSubsetAcrossPathsNot_22() {
	root := hotColdHiddenMutationFixtureRoot()
	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())

	var observed []*agentloop.Loop
	h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "cap-owner",
		SmokeRepoName:    "cap-repo",
		Gateway:          recGW,

		MaxHiddenMutationJudgments: 16,
		ObserveLoop: func(loop *agentloop.Loop) {
			observed = append(observed, loop)
		},
	})

	w := newCaptureWriter()
	completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
		RepoOwner: "cap-owner",
		RepoName:  "cap-repo",
	}), w)
	Expect(err).NotTo(HaveOccurred())
	Expect(completion).NotTo(BeNil())

	_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
	Expect(detHidden).To(Equal(28),
		"fixture must be 20+3+3+2 deterministic hidden_input_mutation signals; got %d", detHidden)
	Expect(agentHidden).To(Equal(16),
		"cap must limit agent hidden_mutation rows to max_hidden_mutation_judgments; got %d", agentHidden)

	Expect(detHidden).To(BeNumerically(">", agentHidden))

	byPath := agentHiddenPathsViaJudgedRefs(w.findings)
	Expect(byPath).NotTo(BeEmpty(), "must attribute agent rows to paths via finding_ref/payload_hash")

	hot := byPath["hot.go"]
	coldA := byPath["cold_a.go"]
	coldB := byPath["cold_b.go"]
	coldC := byPath["cold_c.go"]

	Expect(hot).To(BeNumerically("<", 16),
		"round-robin must not consume entire cap from hot path; paths=%v", byPath)
	Expect(coldA+coldB+coldC).To(BeNumerically(">=", 1),
		"at least one cold-path finding must be judged; paths=%v", byPath)

	Expect(coldA).To(Equal(3), "cold_a should be fully covered under cap=16 RR; paths=%v", byPath)
	Expect(coldB).To(Equal(3), "cold_b should be fully covered under cap=16 RR; paths=%v", byPath)
	Expect(coldC).To(Equal(2), "cold_c should be fully covered under cap=16 RR; paths=%v", byPath)
	Expect(hot).To(Equal(8), "remaining cap after cold paths goes to hot; paths=%v", byPath)

	found, msg := hasJudgmentCapDiagnostic(w.diagnostics)
	Expect(found).To(BeTrue(),
		"must record judgment_cap_omitted diagnostic with selected/omitted; got %#v", w.diagnostics)
	Expect(msg).To(ContainSubstring("selected=16"))
	Expect(msg).To(ContainSubstring("omitted=12"))

	// Cap applies before packing: judged finding_refs across packs == 16.
	var judgedRefs int
	for _, loop := range observed {
		if loop == nil {
			continue
		}
		(&sigbodyhandlerBaselineJudgmentCapAcceptanceTestjudgesARoundR{judgedRefs: &judgedRefs, loop: loop}).call()

	}
	Expect(judgedRefs).To(Equal(16),
		"only the prioritized subset is packed/judged; packed items=%d", judgedRefs)
}

func body_handlerBaselineJudgmentCapAcceptanceTest_176(pkg, path string, n int, root string) {
	GinkgoHelper()
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\ntype S struct {\n", pkg)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "\tF%d string\n", i)
	}
	b.WriteString("}\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func M%d(s *S, v string) { s.F%d = v }\n", i, i)
	}
	Expect(os.WriteFile(filepath.Join(root, path), []byte(b.String()), 0o644)).To(Succeed())
}
