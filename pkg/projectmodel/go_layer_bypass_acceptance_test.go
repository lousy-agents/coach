package projectmodel_test

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func hasLayerBypassDiagnostic(diags []projectmodel.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

var _ = Describe("BuildGoLayerBypass", func() {
	requiredServiceLayer := projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}}

	When("a handler only reaches the pinned database sink through the required layer", func() {
		It("produces zero witnesses and reports a fully evaluated, complete search", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_compliant_only")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "a compliant-only path must never produce a bypass witness, got %+v", result.Witnesses)
			Expect(result.Coverage.Complete).To(BeTrue())
		})

		It("builds one SSA program for the module root rather than one per internal walk", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_compliant_only")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeTrue())
			Expect(result.Coverage.Counts).To(HaveKeyWithValue("ssa_programs_built", 1),
				"call-graph, source identification, and node-directory walks must share one SSA program per root, got counts=%v", result.Coverage.Counts)
		})
	})

	When("a handler reaches the sink both through the required layer and directly", func() {
		It("produces exactly one witness whose Path reflects the direct bypass edge, not the compliant one", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_compliant_and_bypass")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "expected exactly one bypass witness, got %+v", result.Witnesses)
			witness := result.Witnesses[0]

			Expect(witness.ID).To(Equal("bypass:service:example.com/layerbypassmixed.Handler->(*database/sql.DB).Query@" + projectmodel.LayerBypassAlgorithm))
			Expect(witness.Source).To(Equal("example.com/layerbypassmixed.Handler"))
			Expect(witness.Sink).To(Equal("(*database/sql.DB).Query"))
			Expect(witness.RequiredLayer).To(Equal("service"))
			Expect(witness.Confidence).To(Equal(projectmodel.LayerBypassConfidenceHigh))
			Expect(witness.AlgorithmVersion).To(Equal(projectmodel.LayerBypassAlgorithm))

			// False-green control: the fixture's compliant route
			// (Handler -> service.LoadUser -> sink, 3 hops) is strictly
			// shorter than its bypass route (Handler -> directQuery ->
			// rawQuery -> sink, 4 hops), so a broken implementation that
			// leaves the required layer's nodes in the graph (a no-op
			// removal) would have BFS pick the shorter, still-present
			// compliant route instead -- failing this exact-path assertion
			// rather than happening to agree with it.
			Expect(witness.Path).To(HaveLen(4), "expected Handler, directQuery, rawQuery, and the sink, got %+v", witness.Path)
			Expect(witness.Path[0].NodeID).To(Equal("example.com/layerbypassmixed.Handler"))
			Expect(witness.Path[1].NodeID).To(Equal("example.com/layerbypassmixed.directQuery"))
			Expect(witness.Path[2].NodeID).To(Equal("example.com/layerbypassmixed.rawQuery"))
			Expect(witness.Path[3].NodeID).To(Equal("(*database/sql.DB).Query"))
			for _, step := range witness.Path {
				Expect(step.NodeID).NotTo(ContainSubstring("/service."), "witness path must never route through the required service layer, got %+v", witness.Path)
			}

			// Every local-function step resolves a real repository-relative
			// declaration position (see
			// testdata/go_layer_bypass_compliant_and_bypass/main.go), not just the
			// SSA node identity -- proving BuildGoLayerBypass surfaces real source
			// data rather than requiring a downstream consumer to fabricate one.
			Expect(witness.Path[0].Path).To(Equal("main.go"))
			Expect(witness.Path[0].Line).To(Equal(17), "expected Handler's func declaration line")
			Expect(witness.Path[1].Path).To(Equal("main.go"))
			Expect(witness.Path[1].Line).To(Equal(22), "expected directQuery's func declaration line")
			Expect(witness.Path[2].Path).To(Equal("main.go"))
			Expect(witness.Path[2].Line).To(Equal(26), "expected rawQuery's func declaration line")
			// The sink is a stdlib function with no declaration in the snapshot,
			// so it must never carry a fabricated position.
			Expect(witness.Path[3].Path).To(BeEmpty())
			Expect(witness.Path[3].Line).To(BeZero())

			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("a handler reaches the sink only directly, with no coexisting compliant route", func() {
		It("still produces exactly one witness", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_bypass_only")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "a bypass witness must never require a coexisting compliant path, got %+v", result.Witnesses)
			witness := result.Witnesses[0]
			Expect(witness.Source).To(Equal("example.com/layerbypassonly.Handler"))
			Expect(witness.Sink).To(Equal("(*database/sql.DB).Query"))
			Expect(witness.Path[len(witness.Path)-2].NodeID).To(Equal("example.com/layerbypassonly.queryDB"))
		})
	})

	When("a handler has two distinct, equal-length bypass routes to the sink", func() {
		It("produces exactly one witness, following the lexicographically-first next hop", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_dual_bypass")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "BuildGoLayerBypass returns the single shortest witness per source/sink pair, not every bypass route, got %+v", result.Witnesses)
			witness := result.Witnesses[0]

			Expect(witness.Source).To(Equal("example.com/layerbypassdual.Handler"))
			Expect(witness.Sink).To(Equal("(*database/sql.DB).Query"))
			Expect(witness.RequiredLayer).To(Equal("service"))

			// The fixture's two routes -- Handler -> AlphaQuery -> sink and
			// Handler -> BetaQuery -> sink -- are the same length, so
			// bfsShortestPaths' tie-break decides the winner: adjacency lists
			// are sorted by each node's full ssa.Function.RelString(nil)
			// identity (buildCallGraphAdjacency), and bfsShortestPaths
			// enqueues each node at most once, on first discovery. Handler's
			// sorted neighbors are AlphaQuery, BetaQuery, then
			// service.Unused ("example.com/layerbypassdual.AlphaQuery" <
			// "example.com/layerbypassdual.BetaQuery" <
			// "example.com/layerbypassdual/service.Unused", since '.'
			// (0x2E) sorts before '/' (0x2F) at the position right after
			// "layerbypassdual", and 'A' < 'B' between the first two). So
			// AlphaQuery is enqueued and reaches the sink first; BetaQuery's
			// route to the already-visited sink is discovered second and
			// discarded, and service.Unused's edges are removed from
			// adjacency entirely as a required-layer node. A wrong or
			// unstable tie-break (e.g. Go map iteration order, or picking
			// BetaQuery) would fail this exact-path assertion.
			Expect(witness.Path).To(HaveLen(3), "expected Handler, AlphaQuery, and the sink, got %+v", witness.Path)
			Expect(witness.Path[0].NodeID).To(Equal("example.com/layerbypassdual.Handler"))
			Expect(witness.Path[1].NodeID).To(Equal("example.com/layerbypassdual.AlphaQuery"))
			Expect(witness.Path[2].NodeID).To(Equal("(*database/sql.DB).Query"))

			Expect(witness.Path[0].Path).To(Equal("main.go"))
			Expect(witness.Path[0].Line).To(Equal(26), "expected Handler's func declaration line")
			Expect(witness.Path[1].Path).To(Equal("main.go"))
			Expect(witness.Path[1].Line).To(Equal(32), "expected AlphaQuery's func declaration line")

			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("the call graph contains a cycle unrelated to the shortest bypass path", func() {
		It("still finds the correct witness deterministically without hanging", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_layer_bypass_cycle")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "expected exactly one witness despite the unrelated cycle, got %+v", result.Witnesses)
			Expect(result.Witnesses[0].Source).To(Equal("example.com/layerbypasscycle.Handler"))
			Expect(result.Witnesses[0].Path[len(result.Witnesses[0].Path)-2].NodeID).To(Equal("example.com/layerbypasscycle.queryDB"))

			// A cycle that infinite-loops the BFS or double-visits nodes
			// would inflate this well past the handful of local functions
			// this fixture actually has (Handler, cycleA, cycleB, queryDB,
			// service.Unused).
			Expect(result.Coverage.Counts["search_nodes_visited"]).To(BeNumerically("<", 20))
		}, SpecTimeout(20*time.Second))
	})

	When("the only surviving route to the sink is through an unresolved interface dispatch", func() {
		It("produces zero witnesses instead of synthesizing a lower-confidence one", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_unresolved")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "an unresolved dispatch must suppress the witness, not synthesize one, got %+v", result.Witnesses)
			Expect(result.Coverage.Complete).To(BeTrue())
			Expect(result.Coverage.Counts["underlying_unresolved_call_sites"]).To(BeNumerically(">", 0),
				"expected this fixture's unresolved interface dispatch to be counted, got %+v", result.Coverage.Counts)
		})
	})

	When("a handler's only route to the pinned database sink passes through a synthetic bound-method-value wrapper whose real target is local to the snapshot", func() {
		It("produces no witness for the pair and marks Coverage.Complete false with the call graph's synthetic-wrapper diagnostic, instead of silently dead-ending", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_bound_method")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(),
				"a call route that dead-ends at a local-targeted synthetic wrapper must not silently synthesize a bypass witness through it, got %+v", result.Witnesses)
			Expect(result.Coverage.Complete).To(BeFalse(),
				"the underlying call graph's synthetic-wrapper dead end must propagate as layer-bypass incompleteness, not report a silently 'complete' no-witness result")
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagCallUnresolvedSyntheticWrapper)).To(BeTrue(),
				"expected the call graph's project_call_unresolved_synthetic_wrapper diagnostic to surface on LayerBypassResult.Coverage, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
				"a pair searched against an incompletely built call graph must not count as conclusively evaluated")
			Expect(result.Coverage.Counts["source_sink_pairs_truncated"]).To(BeNumerically(">", 0))
			Expect(result.Coverage.Counts["underlying_unresolved_call_sites"]).To(Equal(1),
				"the call graph's unresolved_synthetic_wrapper count must propagate into LayerBypassResult's unresolved-ratio input, not be dropped to zero")
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassBudgetExceeded)).To(BeFalse(),
				"no budget was ever exceeded in this run; a call-graph dead end must not also report a false budget-exceeded diagnostic")
		})
	})

	When("the required layer's prefixes match no package anywhere in the snapshot", func() {
		It("produces zero witnesses and records an ambiguous-layer diagnostic, even though a structural path exists", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_ambiguous")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "an unmatched/malformed required layer must suppress every witness, got %+v", result.Witnesses)
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue(),
				"expected a project_layer_bypass_ambiguous_layer diagnostic, got %+v", result.Coverage.Diagnostics)
			// An ambiguous RequiredLayer means nothing was actually
			// searched: this must not present as a complete, fully
			// evaluated "no bypass found" run.
			Expect(result.Coverage.Complete).To(BeFalse(), "an ambiguous required layer must not report a complete search")
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
				"an ambiguous required layer must not claim any pair was evaluated, got %+v", result.Coverage.Counts)
		})
	})

	When("a caller passes an empty required layer", func() {
		It("suppresses every witness rather than treating an unremoved graph as the bypass search", func() {
			snapshot := os.DirFS("testdata/go_layer_bypass_compliant_and_bypass")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: projectmodel.BypassLayer{},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty())
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue())
			Expect(result.Coverage.Complete).To(BeFalse(), "an ambiguous required layer must not report a complete search")
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
				"an ambiguous required layer must not claim any pair was evaluated, got %+v", result.Coverage.Counts)
		})
	})

	When("the search is interrupted by a node-visitation budget before completion", func() {
		It("produces zero witnesses, marks Coverage.Complete false, and records a truncation diagnostic", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_layer_bypass_compliant_and_bypass")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer:  requiredServiceLayer,
				MaxSearchNodes: 3,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "a budget-truncated pair must never produce a witness, got %+v", result.Witnesses)
			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassBudgetExceeded)).To(BeTrue(),
				"expected a project_layer_bypass_budget_exceeded diagnostic, got %+v", result.Coverage.Diagnostics)
			// MaxSearchNodes: 3 is deliberately the value where the BFS has
			// discovered (enqueued) the sink but not yet dequeued/processed
			// it when the budget check fires -- so a broken evaluator that
			// forgets to gate witness emission on the budget having been hit
			// (skip) would still find and emit a witness here, while a
			// smaller budget wouldn't discriminate between the two (the sink
			// wouldn't be discovered at all either way).
		}, SpecTimeout(20*time.Second))
	})

	When("a wall-time budget expires before the search can run", func() {
		It("returns promptly with Complete false and no witnesses", func(ctx SpecContext) {
			snapshot := os.DirFS("testdata/go_layer_bypass_compliant_and_bypass")
			result, err := projectmodel.BuildGoLayerBypass(context.Background(), snapshot, projectmodel.LayerBypassOptions{
				RequiredLayer: requiredServiceLayer,
				Budgets:       projectmodel.GoBudgets{WallTime: 1 * time.Nanosecond},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Coverage.Complete).To(BeFalse())
			Expect(result.Witnesses).To(BeEmpty())
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassBudgetExceeded)).To(BeTrue())
		}, SpecTimeout(20*time.Second))
	})
})
