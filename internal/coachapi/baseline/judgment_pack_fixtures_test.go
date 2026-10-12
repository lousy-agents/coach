package baseline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// multiHiddenMutationFixtureRoot builds a temp tree with ≥12 hidden_input_mutation
// signals across ≥3 paths and one hot path with ≥6 signals (Story 1 pack fixture).
func multiHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()
	// hot.go: 8 pointer-parameter mutations (hot path ≥6).
	hot := `package hot

type S struct {
	A, B, C, D, E, F, G, H string
}

func M1(s *S, v string) { s.A = v }
func M2(s *S, v string) { s.B = v }
func M3(s *S, v string) { s.C = v }
func M4(s *S, v string) { s.D = v }
func M5(s *S, v string) { s.E = v }
func M6(s *S, v string) { s.F = v }
func M7(s *S, v string) { s.G = v }
func M8(s *S, v string) { s.H = v }
`
	// cold_a.go / cold_b.go: 3 mutations each (cross-file merge eligible under affinity).
	coldA := `package colda

type S struct {
	X, Y, Z string
}

func A1(s *S, v string) { s.X = v }
func A2(s *S, v string) { s.Y = v }
func A3(s *S, v string) { s.Z = v }
`
	coldB := `package coldb

type S struct {
	X, Y, Z string
}

func B1(s *S, v string) { s.X = v }
func B2(s *S, v string) { s.Y = v }
func B3(s *S, v string) { s.Z = v }
`
	Expect(os.WriteFile(filepath.Join(root, "hot.go"), []byte(hot), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "cold_a.go"), []byte(coldA), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "cold_b.go"), []byte(coldB), 0o644)).To(Succeed())
	return root
}

func countFindingsBySource(findings []coachapi.JobFinding) (det, agent, detHidden, agentHidden int) {
	for _, f := range findings {
		switch f.Source {
		case coachapi.FindingSourceDeterministic:
			det++
			if strings.Contains(string(f.Payload), "hidden_input_mutation") ||
				strings.Contains(string(f.Payload), "state.hidden_input_mutation") {
				detHidden++
			}
		case coachapi.FindingSourceAgent:
			agent++
			if f.RubricID != nil && *f.RubricID == rubrics.IDHiddenMutationContextualization {
				agentHidden++
			}
		}
	}
	return det, agent, detHidden, agentHidden
}

func countHiddenMutationToolCalls(loops []*agentloop.Loop) int {
	n := 0
	for _, loop := range loops {
		if loop == nil {
			continue
		}
		n += handlerHiddenMutationCallCount(loop)
	}
	return n
}

// handlerHiddenMutationCallCount counts the handler-sourced
// hidden_mutation_contextualization calls one loop recorded.
func handlerHiddenMutationCallCount(loop *agentloop.Loop) int {
	n := 0
	for _, c := range loop.Calls() {
		if c.Source == agentloop.CallSourceHandler && c.Name == rubrics.IDHiddenMutationContextualization {
			n++
		}
	}
	return n
}

// hasMultiItemHiddenMutationPack reports whether loop recorded a
// hidden_mutation_contextualization call whose args carried two or more
// pack items.
func hasMultiItemHiddenMutationPack(loop *agentloop.Loop) bool {
	saw := false
	for _, c := range loop.Calls() {
		if c.Name != rubrics.IDHiddenMutationContextualization {
			continue
		}
		var args struct {
			Items []json.RawMessage `json:"items"`
		}
		if json.Unmarshal(c.Args, &args) == nil && len(args.Items) >= 2 {
			saw = true
		}
	}
	return saw
}
