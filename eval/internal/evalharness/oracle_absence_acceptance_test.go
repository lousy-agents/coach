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
	DescribeTable("rejects at the g2 step, naming the symbol or the edge, when the last hop of the route is the one that is missing (AC-30, AC-31)",
		func(cell int, rule string, mutate func(generated *evalgen.Program, last int), subject func(generated evalgen.Program, last int) string) {
			generated := cloned(program(cell))
			last := len(generated.Manifest.Template.APISurface) - 1
			Expect(last).To(BeNumerically(">=", 2), "a route of at least three symbols")
			mutate(&generated, last)

			_, err := evalharness.GradeOracles(generated)

			Expect(err).To(HaveOccurred())
			Expect(rejectionStep(err)).To(Equal(evalgen.StepG2))
			Expect(err.Error()).To(ContainSubstring(rule))
			Expect(err.Error()).To(ContainSubstring(subject(generated, last)))
		},
		Entry("the last symbol's function is gone from the first oracle", 0, "api_surface.symbol_absent",
			func(g *evalgen.Program, last int) {
				d, _ := surfaceSymbol(*g, last)
				g.Solution[entryFileOf(d)] = emptyPackage(d)
			},
			func(g evalgen.Program, last int) string { d, n := surfaceSymbol(g, last); return d + "." + n }),
		Entry("the last hop no longer imports the package before it is reached, on a route of three", 0, "architecture.required_edge_absent",
			func(g *evalgen.Program, last int) {
				d, n := surfaceSymbol(*g, last-1)
				g.Solution[entryFileOf(d)] = declaredWithoutBody(d, n)
			},
			func(g evalgen.Program, last int) string {
				from, _ := surfaceSymbol(g, last-1)
				to, _ := surfaceSymbol(g, last)
				return from + "->" + to
			}),
		Entry("a hop in the middle of a route of four no longer imports the next one", 12, "architecture.required_edge_absent",
			func(g *evalgen.Program, _ int) {
				d, n := surfaceSymbol(*g, 1)
				g.Solution[entryFileOf(d)] = declaredWithoutBody(d, n)
			},
			func(g evalgen.Program, _ int) string {
				from, _ := surfaceSymbol(g, 1)
				to, _ := surfaceSymbol(g, 2)
				return from + "->" + to
			}),
	)

	It("rejects at the g2 step, instead of panicking, when the manifest names a symbol with no package qualifier (AC-30)", func() {
		generated := cloned(program(0))
		generated.Manifest.Template.APISurface = append([]evalgen.APISymbol(nil), generated.Manifest.Template.APISurface...)
		generated.Manifest.Template.APISurface[0].Symbol = "nodot"

		_, err := evalharness.GradeOracles(generated)

		Expect(err).To(HaveOccurred())
		Expect(rejectionStep(err)).To(Equal(evalgen.StepG2))
		Expect(err.Error()).To(ContainSubstring("api_surface.symbol_absent:nodot.\n"), "the subject names the symbol as the manifest wrote it")
	})
})
