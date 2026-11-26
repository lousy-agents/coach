package evalharness_test

import (
	"path"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

// surfaceSymbol splits the manifest's i-th api_surface symbol into the package directory and the function name.
func surfaceSymbol(generated evalgen.Program, index int) (directory, name string) {
	qualified := strings.TrimPrefix(generated.Manifest.Template.APISurface[index].Symbol, moduleOf(generated)+"/")
	split := strings.LastIndex(qualified, ".")
	return qualified[:split], qualified[split+1:]
}

func entryFileOf(directory string) string { return directory + "/" + path.Base(directory) + ".go" }

var _ = Describe("the oracles' api_surface and required-edge checks (design §4, G2)", func() {
	declaredWithoutBody := func(directory, name string) []byte {
		return []byte("package " + path.Base(directory) + "\n\nfunc " + name + "() {}\n")
	}
	emptyPackage := func(directory string) []byte { return []byte("package " + path.Base(directory) + "\n") }

	DescribeTable("rejects at the g2 step, naming the rule and the symbol or the edge, when an oracle lacks what the manifest requires (AC-30, AC-31)",
		func(rule, subject func(generated evalgen.Program) string, mutate func(generated *evalgen.Program)) {
			generated := cloned(program(0))
			mutate(&generated)

			_, err := evalharness.GradeOracles(generated)

			Expect(err).To(HaveOccurred())
			Expect(rejectionStep(err)).To(Equal(evalgen.StepG2))
			Expect(err.Error()).To(ContainSubstring(rule(generated)))
			Expect(err.Error()).To(ContainSubstring(subject(generated)))
		},
		Entry("the entry's function is gone from the first oracle",
			func(evalgen.Program) string { return "api_surface.symbol_absent" },
			func(g evalgen.Program) string { d, n := surfaceSymbol(g, 0); return d + "." + n },
			func(g *evalgen.Program) { d, _ := surfaceSymbol(*g, 0); g.Solution[entryFileOf(d)] = emptyPackage(d) }),
		Entry("the entry's function is gone from the follow-on oracle only",
			func(evalgen.Program) string { return "api_surface.symbol_absent" },
			func(g evalgen.Program) string { d, n := surfaceSymbol(g, 0); return d + "." + n },
			func(g *evalgen.Program) {
				d, _ := surfaceSymbol(*g, 0)
				g.FollowOnSolution[entryFileOf(d)] = emptyPackage(d)
			}),
		Entry("a hop's function is gone from the first oracle",
			func(evalgen.Program) string { return "api_surface.symbol_absent" },
			func(g evalgen.Program) string { d, n := surfaceSymbol(g, 1); return d + "." + n },
			func(g *evalgen.Program) { d, _ := surfaceSymbol(*g, 1); g.Solution[entryFileOf(d)] = emptyPackage(d) }),
		Entry("the entry no longer imports the first hop in the first oracle, though its function is declared",
			func(evalgen.Program) string { return "architecture.required_edge_absent" },
			func(g evalgen.Program) string {
				from, _ := surfaceSymbol(g, 0)
				to, _ := surfaceSymbol(g, 1)
				return from + "->" + to
			},
			func(g *evalgen.Program) {
				d, n := surfaceSymbol(*g, 0)
				g.Solution[entryFileOf(d)] = declaredWithoutBody(d, n)
			}),
		Entry("the entry no longer imports the first hop in the follow-on oracle only",
			func(evalgen.Program) string { return "architecture.required_edge_absent" },
			func(g evalgen.Program) string {
				from, _ := surfaceSymbol(g, 0)
				to, _ := surfaceSymbol(g, 1)
				return from + "->" + to
			},
			func(g *evalgen.Program) {
				d, n := surfaceSymbol(*g, 0)
				g.FollowOnSolution[entryFileOf(d)] = declaredWithoutBody(d, n)
			}),
	)
})
