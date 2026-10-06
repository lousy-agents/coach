package baseline_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectRoundRobinJudgmentCapAcrossPaths() {
	root := hotColdHiddenMutationFixtureRoot()
	recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())

	var observed []*agentloop.Loop
	h := baseline.NewScanHandler(baseline.ScanConfig{
		SmokeFixturePath: root,
		SmokeRepoOwner:   "cap-owner",
		SmokeRepoName:    "cap-repo",
		Gateway:          recGW,
		// Explicit default-equivalent cap so the test documents the knob.
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
	// Deterministic findings remain complete regardless of cap.
	Expect(detHidden).To(BeNumerically(">", agentHidden))

	byPath := agentHiddenPathsViaJudgedRefs(w.findings)
	Expect(byPath).NotTo(BeEmpty(), "must attribute agent rows to paths via finding_ref/payload_hash")

	hot := byPath["hot.go"]
	coldA := byPath["cold_a.go"]
	coldB := byPath["cold_b.go"]
	coldC := byPath["cold_c.go"]
	// Must NOT take all 16 from the hot path when cold paths exist.
	Expect(hot).To(BeNumerically("<", 16),
		"round-robin must not consume entire cap from hot path; paths=%v", byPath)
	Expect(coldA+coldB+coldC).To(BeNumerically(">=", 1),
		"at least one cold-path finding must be judged; paths=%v", byPath)
	// Binding policy exhausts cold paths first under RR: 3+3+2 cold + 8 hot.
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
		judgedRefs += judgedHiddenMutationRefs(loop)
	}
	Expect(judgedRefs).To(Equal(16),
		"only the prioritized subset is packed/judged; packed items=%d", judgedRefs)
}

// judgedHiddenMutationRefs counts the finding refs loop sent to
// hidden_mutation_contextualization: a pack call counts its items, a
// singular call counts one.
func judgedHiddenMutationRefs(loop *agentloop.Loop) int {
	refs := 0
	for _, c := range loop.Calls() {
		if c.Name != rubrics.IDHiddenMutationContextualization {
			continue
		}
		var args struct {
			Items []json.RawMessage `json:"items"`
		}
		if json.Unmarshal(c.Args, &args) == nil && len(args.Items) > 0 {
			refs += len(args.Items)
			continue
		}
		refs++
	}
	return refs
}
