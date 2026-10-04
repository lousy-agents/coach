package projectmodel_test

import (
	"context"
	"os"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_goCallgraphAcceptanceTest_emitsNoCallFactForThatSiteButRecordsAnUnresolved_52() {
	snapshot := os.DirFS("testdata/go_callgraph_interface")
	result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, f := range result.CallFacts {
		Expect(f.To).NotTo(ContainSubstring("Greet"), "an interface dispatch must not contribute a CallFact, got %+v", result.CallFacts)
	}
	Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedInterface)).To(BeTrue(),
		"expected a project_call_unresolved_interface diagnostic, got %+v", result.Coverage.Diagnostics)
	Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_interface", 1))
}

func body_goCallgraphAcceptanceTest_emitsNoCallFactForTheReflectedTargetButRecordsAn_79() {
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
}

func body_goCallgraphAcceptanceTest_doesNotSilentlyDeadEndAtTheWrapperItReportsAnExp_145() {
	snapshot := os.DirFS("testdata/go_callgraph_synthetic_wrapper")
	result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, f := range result.CallFacts {
		Expect(f.To).NotTo(ContainSubstring("$bound"),
			"a call resolving to a synthetic bound-method wrapper must not surface as a CallFact whose target is never walked, got %+v", result.CallFacts)
	}
	Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedSyntheticWrapper)).To(BeTrue(),
		"expected a project_call_unresolved_synthetic_wrapper diagnostic for the call into the bound-method wrapper, got %+v", result.Coverage.Diagnostics)
	Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_synthetic_wrapper", 1))
	Expect(result.Coverage.Complete).To(BeFalse(),
		"a call site that dead-ends at a synthetic wrapper must mark the root's contribution incomplete, not leave Coverage.Complete true")
}

func body_goCallgraphAcceptanceTest_doesNotTreatTheDeadEndAsALostLocalEdgeTheWrapper_163() {
	snapshot := os.DirFS("testdata/go_callgraph_synthetic_wrapper_external")
	result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
	Expect(err).NotTo(HaveOccurred())

	Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedSyntheticWrapper)).To(BeFalse(),
		"a wrapper whose real target is outside the snapshot was never going to be walked either way -- it must not be misclassified as a lost local edge, got %+v", result.Coverage.Diagnostics)
	Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_synthetic_wrapper", 0))

	found := false
	for _, f := range result.CallFacts {
		if f.From == "example.com/callgraphsyntheticwrapperexternal.CallExternalBound" && strings.Contains(f.To, "Done") {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "expected a CallFact from CallExternalBound into the external bound-method wrapper, got %+v", result.CallFacts)

	Expect(result.Coverage.Complete).To(BeTrue(),
		"a call site whose synthetic wrapper targets a function outside the snapshot must not flip Coverage.Complete false")
}

func body_goCallgraphAcceptanceTest_routesEachCallSiteToTheGenericOriginSOwnIdentity_186() {
	snapshot := os.DirFS("testdata/go_callgraph_generic")
	result, err := projectmodel.BuildGoCallGraph(context.Background(), snapshot, projectmodel.CallGraphOptions{})
	Expect(err).NotTo(HaveOccurred())

	Expect(hasCallGraphDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedSyntheticWrapper)).To(BeFalse(),
		"a call into a local generic function's own instantiation must not be treated as an unresolved synthetic wrapper, got %+v", result.Coverage.Diagnostics)
	Expect(result.Coverage.Counts).To(HaveKeyWithValue("unresolved_synthetic_wrapper", 0))
	Expect(result.Coverage.Complete).To(BeTrue(),
		"a local generic call must not silently zero out Coverage.Complete for the whole snapshot")

	genericEdgeCount := 0
	for _, f := range result.CallFacts {
		if f.From == "example.com/callgraphgeneric.CallGeneric" {
			Expect(f.To).To(Equal("example.com/callgraphgeneric.Identity"),
				"both Identity[int] and Identity[string] call sites must route to the same origin identity, got %+v", result.CallFacts)
			genericEdgeCount++
		}
	}
	Expect(genericEdgeCount).To(Equal(2), "expected one CallFact per instantiated call site, both routed to the same origin identity, got %+v", result.CallFacts)

	Expect(hasCallFact(result.CallFacts,
		"example.com/callgraphgeneric.Identity",
		"example.com/callgraphgeneric.Sink",
	)).To(BeTrue(), "expected the generic origin's own outgoing call to still be walked directly, got %+v", result.CallFacts)
}
