package coachapi_test

import (
	"context"
	"encoding/json"
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

// hotColdHiddenMutationFixtureRoot builds a temp tree with one hot path and
// several cold paths (20 + 3 + 3 + 2) for Story 3 priority-cap round-robin.
func hotColdHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()

	writeMutators := func(pkg, path string, n int) {
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

	writeMutators("hot", "hot.go", 20)
	writeMutators("colda", "cold_a.go", 3)
	writeMutators("coldb", "cold_b.go", 3)
	writeMutators("coldc", "cold_c.go", 2)
	return root
}

// agentHiddenPathsViaJudgedRefs maps agent rows back to deterministic paths using
// PayloadHash as finding_ref (pack path stores deterministic hash on agent row).
func agentHiddenPathsViaJudgedRefs(findings []coachapi.JobFinding) map[string]int {
	detByHash := map[string]string{}
	for _, f := range findings {
		if f.Source != coachapi.FindingSourceDeterministic {
			continue
		}
		if !strings.Contains(string(f.Payload), "hidden_input_mutation") {
			continue
		}
		var sig struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(f.Payload, &sig) != nil || sig.Path == "" {
			continue
		}
		detByHash[f.PayloadHash] = sig.Path
	}
	paths := map[string]int{}
	for _, f := range findings {
		if f.Source != coachapi.FindingSourceAgent {
			continue
		}
		if f.RubricID == nil || *f.RubricID != rubrics.IDHiddenMutationContextualization {
			continue
		}
		if p, ok := detByHash[f.PayloadHash]; ok {
			paths[p]++
			continue
		}
		// Fallback: finding_ref inside payload.
		var body struct {
			FindingRef string `json:"finding_ref"`
			Path       string `json:"path"`
		}
		_ = json.Unmarshal(f.Payload, &body)
		if body.Path != "" {
			paths[body.Path]++
			continue
		}
		if p, ok := detByHash[body.FindingRef]; ok {
			paths[p]++
		}
	}
	return paths
}

func hasJudgmentCapDiagnostic(diags []coachapi.JobDiagnostic) (found bool, message string) {
	for _, d := range diags {
		msg := d.Message
		scope := strings.ToLower(d.Scope)
		if strings.Contains(msg, "judgment_cap_omitted") ||
			scope == "judgment_cap" ||
			(strings.Contains(msg, "selected=") && strings.Contains(msg, "omitted=")) {
			return true, d.Message
		}
	}
	return false, ""
}

var _ = Describe("repo_baseline_scan judgment priority cap (local-LLM Story 3)", func() {
	When("deterministic hidden_mutation count exceeds max_hidden_mutation_judgments", func() {
		It("judges a round-robin prioritized subset across paths, not the entire cap from the hot path, and records selected/omitted diagnostic", func() {
			root := hotColdHiddenMutationFixtureRoot()
			recGW := newRecordingJudgeGateway(modelgateway.NewStubGateway())

			var observed []*agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
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
				for _, c := range loop.Calls() {
					if c.Name != rubrics.IDHiddenMutationContextualization {
						continue
					}
					var args struct {
						Items []json.RawMessage `json:"items"`
					}
					if json.Unmarshal(c.Args, &args) == nil && len(args.Items) > 0 {
						judgedRefs += len(args.Items)
						continue
					}
					// Singular envelope.
					judgedRefs++
				}
			}
			Expect(judgedRefs).To(Equal(16),
				"only the prioritized subset is packed/judged; packed items=%d", judgedRefs)
		})
	})

	When("deterministic hidden_mutation count is under the judgment cap", func() {
		It("judges all hidden_mutation signals and does not record a judgment cap diagnostic", func() {
			root := multiHiddenMutationFixtureRoot() // 8+3+3 = 14 < default 16

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "cap-owner",
				SmokeRepoName:    "cap-repo",
				Gateway:          modelgateway.NewStubGateway(),
				// Zero → default 16; fixture has 14.
				MaxHiddenMutationJudgments: 0,
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "cap-owner",
				RepoName:  "cap-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(detHidden).To(BeNumerically(">=", 12))
			Expect(detHidden).To(BeNumerically("<=", 16),
				"under-cap fixture must stay at or below default cap; got %d", detHidden)
			Expect(agentHidden).To(Equal(detHidden),
				"when under cap every hidden_mutation signal is judged")

			found, msg := hasJudgmentCapDiagnostic(w.diagnostics)
			Expect(found).To(BeFalse(),
				"no cap diagnostic when nothing omitted; got %q in %#v", msg, w.diagnostics)
		})
	})

	When("MaxHiddenMutationJudgments is negative (unlimited)", func() {
		It("judges all hidden_mutation signals even when count exceeds the default 16", func() {
			root := hotColdHiddenMutationFixtureRoot() // 28 findings

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath:           root,
				SmokeRepoOwner:             "cap-owner",
				SmokeRepoName:              "cap-repo",
				Gateway:                    modelgateway.NewStubGateway(),
				MaxHiddenMutationJudgments: -1,
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "cap-owner",
				RepoName:  "cap-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			_, _, detHidden, agentHidden := countFindingsBySource(w.findings)
			Expect(detHidden).To(Equal(28))
			Expect(agentHidden).To(Equal(28),
				"negative max means unlimited; all findings judged")

			found, _ := hasJudgmentCapDiagnostic(w.diagnostics)
			Expect(found).To(BeFalse(), "unlimited must not emit cap diagnostic")
		})
	})
})
