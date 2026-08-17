package projectmodel_test

import (
	"context"
	"encoding/json"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func hasCallGraphDiagnostic(diags []projectmodel.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func hasCallFact(facts []projectmodel.CallFact, from, to string) bool {
	for _, f := range facts {
		if f.From == from && f.To == to {
			return true
		}
	}
	return false
}

var _ = Describe("BuildGoCallGraph", func() {
	When("a function calls a function in a different package directly", func() {
		It("resolves exactly the expected CallFact and records algorithm/backend provenance", func() {
			snapshot := os.DirFS("testdata/go_callgraph_direct")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallFact(result.CallFacts,
				"example.com/callgraphdirect/pkga.Caller",
				"example.com/callgraphdirect/pkgb.Callee",
			)).To(BeTrue(), "expected a direct CallFact from pkga.Caller to pkgb.Callee, got %+v", result.CallFacts)

			Expect(result.Algorithm).NotTo(BeEmpty(), "expected an observable algorithm/backend provenance identifier")
			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("a call site dispatches through an interface value", func() {
		It("emits no CallFact for that site but records an unresolved-interface diagnostic", func() {
			snapshot := os.DirFS("testdata/go_callgraph_interface")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, f := range result.CallFacts {
				Expect(f.To).NotTo(ContainSubstring("Greet"), "an interface dispatch must not contribute a CallFact, got %+v", result.CallFacts)
			}
			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedInterface)).To(BeTrue(),
				"expected a project_call_unresolved_interface diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_interface", 1))
		})
	})

	When("a call site invokes a func-typed parameter", func() {
		It("emits no CallFact for that site but records an unresolved-function-value diagnostic", func() {
			snapshot := os.DirFS("testdata/go_callgraph_function_value")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedFunctionValue)).To(BeTrue(),
				"expected a project_call_unresolved_function_value diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_function_value", 1))
		})
	})

	When("a call site dispatches through reflect.Value.Call", func() {
		It("emits no CallFact for the reflected target but records an unresolved-reflection diagnostic", func() {
			snapshot := os.DirFS("testdata/go_callgraph_reflection")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, f := range result.CallFacts {
				Expect(f.To).NotTo(Equal("example.com/callgraphreflection.Target"),
					"a reflection-dispatched call must not contribute a direct CallFact to its target, got %+v", result.CallFacts)
			}
			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedReflection)).To(BeTrue(),
				"expected a project_call_unresolved_reflection diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_reflection", 1))
		})
	})

	When("a function/handler value is passed to a well-known HTTP registration call rather than invoked directly", func() {
		It("records an unresolved-framework-registration diagnostic for the handler argument", func() {
			snapshot := os.DirFS("testdata/go_callgraph_framework_registration")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedFrameworkRegistration)).To(BeTrue(),
				"expected a project_call_unresolved_framework_registration diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_framework_registration", 1))
		})

		It("keeps callgraph_static_nodes on the snapshot's own functions instead of SSA-building the standard library", func() {
			snapshot := os.DirFS("testdata/go_callgraph_framework_registration")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedFrameworkRegistration)).To(BeTrue(),
				"stdlib types must still be available so http.HandleFunc is classified, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts["functions_seen"]).To(BeNumerically("<=", 10),
				"fixture only has a handful of local functions, got functions_seen=%d", result.Coverage.Counts["functions_seen"])
			Expect(result.Coverage.Counts["callgraph_static_nodes"]).To(BeNumerically("<", 5000),
				"LoadAllSyntax of net/http produces ~15k static-graph nodes from stdlib function bodies; LoadSyntax should stay on the fixture plus export-data stubs, got %d", result.Coverage.Counts["callgraph_static_nodes"])
		})
	})

	When("a handler value is registered via http.Handle", func() {
		It("records an unresolved-framework-registration diagnostic even though the parameter type is the Handler interface, not a func", func() {
			snapshot := os.DirFS("testdata/go_callgraph_framework_registration_handle")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedFrameworkRegistration)).To(BeTrue(),
				"expected a project_call_unresolved_framework_registration diagnostic for http.Handle, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_framework_registration", 1))
		})
	})

	When("a handler value is registered via (*http.ServeMux).Handle", func() {
		It("counts exactly one unresolved-framework-registration diagnostic, not one per call argument including the receiver", func() {
			snapshot := os.DirFS("testdata/go_callgraph_framework_registration_mux")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedFrameworkRegistration)).To(BeTrue(),
				"expected a project_call_unresolved_framework_registration diagnostic for mux.Handle, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_framework_registration", 1),
				"the *http.ServeMux receiver in Args[0] must not itself be misclassified as a handler argument")
		})
	})

	When("a package in the snapshot fails to type-check", func() {
		It("marks Complete false and records a build-failed diagnostic instead of silently looking complete", func() {
			snapshot := os.DirFS("testdata/go_callgraph_typecheck_failure")
			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeFalse(),
				"a package that fails to type-check must not leave Coverage.Complete true")
			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallGraphBuildFailed)).To(BeTrue(),
				"expected a project_callgraph_build_failed diagnostic, got %+v", result.Coverage.Diagnostics)
		})
	})

	When("a context is already cancelled before the build starts", func() {
		It("returns promptly with Complete false and a budget-exceeded diagnostic instead of hanging", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_callgraph_budget")
			cancelledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			result, err := projectmodel.BuildGoCallGraph(cancelledCtx, snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallGraphBudgetExceeded)).To(BeTrue(),
				"expected a project_callgraph_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
		}, SpecTimeout(10*time.Second))
	})

	When("a graph-edge budget is smaller than the number of resolvable call sites", func() {
		It("truncates deterministically, marks Complete false, and stops within limits", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_callgraph_budget")

			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{
				Budgets: projectmodel.GoBudgets{MaxGraphEdges: 1},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(len(result.CallFacts)).To(BeNumerically("<=", 1))
			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallGraphBudgetExceeded)).To(BeTrue(),
				"expected a project_callgraph_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
		}, SpecTimeout(10*time.Second))
	})

	When("a graph-node budget is smaller than the number of local functions", func() {
		It("truncates deterministically, marks Complete false, and stops within limits", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_callgraph_budget")

			result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{
				Budgets: projectmodel.GoBudgets{MaxGraphNodes: 1},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("functions_seen", 1))
			Expect(len(result.CallFacts)).To(BeNumerically("<=", 1))
			Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallGraphBudgetExceeded)).To(BeTrue(),
				"expected a project_callgraph_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
		}, SpecTimeout(10*time.Second))
	})

	When("the same snapshot is built twice", func() {
		It("produces byte-identical CallFacts ordering and content", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_callgraph_direct")

			first, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())
			second, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			firstJSON, err := json.Marshal(first.CallFacts)
			Expect(err).NotTo(HaveOccurred())
			secondJSON, err := json.Marshal(second.CallFacts)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstJSON).To(Equal(secondJSON))
			Expect(first.CallFacts).To(Equal(second.CallFacts))
		}, SpecTimeout(20*time.Second))

		It("produces byte-identical CallFacts from two different absolute temp roots", func(ctx SpecContext) {
			left := copyFixtureToTempDir("testdata/go_callgraph_direct")
			right := copyFixtureToTempDir("testdata/go_callgraph_direct")
			Expect(left).NotTo(Equal(right))

			leftResult, err := projectmodel.BuildGoCallGraph(context.Background(), os.DirFS(left), projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())
			rightResult, err := projectmodel.BuildGoCallGraph(context.Background(), os.DirFS(right), projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			leftJSON, err := json.Marshal(leftResult.CallFacts)
			Expect(err).NotTo(HaveOccurred())
			rightJSON, err := json.Marshal(rightResult.CallFacts)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftJSON).To(Equal(rightJSON))
			Expect(string(leftJSON)).NotTo(ContainSubstring(left))
			Expect(string(leftJSON)).NotTo(ContainSubstring(right))
		}, SpecTimeout(20*time.Second))

		It("produces byte-identical Coverage, with no temp-root leakage, from two different absolute temp roots when a package fails to type-check", func(ctx SpecContext) {
			left := copyFixtureToTempDir("testdata/go_callgraph_typecheck_failure")
			right := copyFixtureToTempDir("testdata/go_callgraph_typecheck_failure")
			Expect(left).NotTo(Equal(right))

			leftResult, err := projectmodel.BuildGoCallGraph(context.Background(), os.DirFS(left), projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())
			rightResult, err := projectmodel.BuildGoCallGraph(context.Background(), os.DirFS(right), projectmodel.CallGraphOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(hasCallGraphDiagnostic(leftResult.Coverage.Diagnostics, projectmodel.DiagCallGraphBuildFailed)).To(BeTrue(),
				"expected the type-check failure to produce a build-failed diagnostic to actually exercise the temp-dir-stripping path")

			leftCoverageJSON, err := json.Marshal(leftResult.Coverage)
			Expect(err).NotTo(HaveOccurred())
			rightCoverageJSON, err := json.Marshal(rightResult.Coverage)
			Expect(err).NotTo(HaveOccurred())
			Expect(leftCoverageJSON).To(Equal(rightCoverageJSON))
			Expect(string(leftCoverageJSON)).NotTo(ContainSubstring(left))
			Expect(string(leftCoverageJSON)).NotTo(ContainSubstring(right))
		}, SpecTimeout(20*time.Second))
	})
})
