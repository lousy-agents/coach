package projectmodel_test

import (
	"context"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// tsLayerBypassSnapshot extends the shared TS sidecar snapshot with a real
// file under "service/" -- the required layer these specs configure --
// distinct from any call-graph node the fixture handlers emit, so the
// ambiguous-layer guard's file-inventory match reflects a genuine snapshot
// rather than a hand-planted call-graph node.
func tsLayerBypassSnapshot() fstest.MapFS {
	snap := fstest.MapFS{}
	for name, file := range tsSidecarSnapshot() {
		snap[name] = file
	}
	snap["service/inventory.ts"] = &fstest.MapFile{Data: []byte("export const noop = 1;\n")}
	return snap
}

var _ = Describe("BuildTypeScriptLayerBypass", func() {
	requiredServiceLayer := projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}}

	When("a route handler outside the required layer has a resolved call path directly to a pinned sink", func() {
		It("produces exactly one high-confidence witness under the TS algorithm identity", func() {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsLayerBypassSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), requiredServiceLayer)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Algorithm).To(Equal("ts-layer-bypass-registry@1"))
			Expect(result.Algorithm).NotTo(Equal(projectmodel.LayerBypassAlgorithm),
				"expected the TS traversal's own algorithm identity, distinct from Go's go-layer-bypass-registry@1")

			Expect(result.Witnesses).To(HaveLen(1), "expected exactly one bypass witness, got %+v", result.Witnesses)
			witness := result.Witnesses[0]

			Expect(witness.ID).To(Equal("bypass:service:file:src/handlers/app.ts#getUsers->(PrismaClient).findMany@" + projectmodel.TSLayerBypassAlgorithm))
			Expect(witness.Source).To(Equal("file:src/handlers/app.ts#getUsers"))
			Expect(witness.Sink).To(Equal("(PrismaClient).findMany"))
			Expect(witness.RequiredLayer).To(Equal("service"))
			Expect(witness.Confidence).To(Equal(projectmodel.LayerBypassConfidenceHigh))
			Expect(witness.AlgorithmVersion).To(Equal("ts-layer-bypass-registry@1"))

			Expect(witness.Path).To(Equal([]projectmodel.LayerBypassStep{
				{NodeID: "file:src/handlers/app.ts#getUsers", Path: "src/handlers/app.ts"},
				{NodeID: "(PrismaClient).findMany"},
			}), "expected the source's own repo-relative path resolved from its node ID, and the sink left unpositioned, got %+v", witness.Path)

			Expect(result.Sources).To(Equal([]string{"file:src/handlers/app.ts#getUsers"}))
			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("a route handler's own declaration directory falls under the required layer", func() {
		It("removes the source node from adjacency entirely and produces zero witnesses", func() {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsLayerBypassSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_compliant"), requiredServiceLayer)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "a handler declared inside the required layer must never produce a bypass witness, got %+v", result.Witnesses)
			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("a handler has two distinct, equal-length bypass routes to the sink through a synthetic multi-hop call graph", func() {
		It("produces exactly one witness, following the lexicographically-first next hop", func() {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsLayerBypassSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_dual"), requiredServiceLayer)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "BuildTypeScriptLayerBypass returns the single shortest witness per source/sink pair, not every bypass route, got %+v", result.Witnesses)
			witness := result.Witnesses[0]

			Expect(witness.Source).To(Equal("file:src/app.ts#Handler"))
			Expect(witness.Sink).To(Equal("(PrismaClient).findMany"))
			Expect(witness.RequiredLayer).To(Equal("service"))

			// False-green control: Handler's two routes (via AlphaQuery and
			// BetaQuery) are the same length, so bfsShortestPaths' tie-break
			// decides the winner via sorted adjacency: "file:src/app.ts#AlphaQuery"
			// < "file:src/app.ts#BetaQuery" ('A' < 'B'), so AlphaQuery is
			// enqueued and reaches the sink first. The unrelated
			// "file:service/unused.ts#Unused" edge, sorting before both (since
			// "file:se" < "file:sr"), is removed from adjacency entirely as a
			// required-layer node -- a broken node-removal step would leave it
			// in adjacency without changing this assertion, so the removal is
			// exercised by result.Coverage.Counts below, not by this path shape
			// alone.
			Expect(witness.Path).To(HaveLen(3), "expected Handler, AlphaQuery, and the sink, got %+v", witness.Path)
			Expect(witness.Path[0].NodeID).To(Equal("file:src/app.ts#Handler"))
			Expect(witness.Path[1].NodeID).To(Equal("file:src/app.ts#AlphaQuery"))
			Expect(witness.Path[2].NodeID).To(Equal("(PrismaClient).findMany"))

			Expect(result.Coverage.Counts).To(HaveKeyWithValue("required_layer_nodes_matched", 1),
				"expected exactly the Unused node to be classified under the required layer, got %+v", result.Coverage.Counts)

			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("the call graph contains a cycle unrelated to the shortest bypass path", func() {
		It("still finds the correct witness deterministically without hanging", func(ctx SpecContext) {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsLayerBypassSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_cycle"), requiredServiceLayer)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "expected exactly one witness despite the unrelated cycle, got %+v", result.Witnesses)
			witness := result.Witnesses[0]
			Expect(witness.Source).To(Equal("file:src/app.ts#Handler"))
			Expect(witness.Path).To(HaveLen(3), "expected Handler, QueryDB, and the sink, got %+v", witness.Path)
			Expect(witness.Path[1].NodeID).To(Equal("file:src/app.ts#QueryDB"))

			// A cycle that infinite-loops the BFS or double-visits nodes would
			// inflate this well past the handful of local functions this
			// fixture actually has (Handler, CycleA, CycleB, QueryDB).
			Expect(result.Coverage.Counts["search_nodes_visited"]).To(BeNumerically("<", 20))
		}, SpecTimeout(20*time.Second))
	})

	When("the required layer's prefixes match no node anywhere in the snapshot", func() {
		It("produces zero witnesses and records an ambiguous-layer diagnostic, even though a structural path exists", func() {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "an unmatched/malformed required layer must suppress every witness, got %+v", result.Witnesses)
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue(),
				"expected a project_layer_bypass_ambiguous_layer diagnostic, got %+v", result.Coverage.Diagnostics)
			Expect(result.Coverage.Complete).To(BeFalse(), "an ambiguous required layer must not report a complete search")
			Expect(result.Coverage.Counts["source_sink_pairs_evaluated"]).To(Equal(0),
				"an ambiguous required layer must not claim any pair was evaluated, got %+v", result.Coverage.Counts)
		})
	})

	When("a caller passes an empty required layer", func() {
		It("suppresses every witness rather than treating an unremoved graph as the bypass search", func() {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), projectmodel.BypassLayer{})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty())
			Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue())
			Expect(result.Coverage.Complete).To(BeFalse())
		})
	})

	When("the sidecar emits only the real depth-1 edge shape (a single handler-to-sink call-graph edge, with no same-layer call-graph node) and the required layer's own files exist only in the snapshot's file inventory", func() {
		It("still produces a high-confidence witness, resolving the required layer from the file inventory rather than requiring a call-graph node under it", func() {
			realisticSnapshot := fstest.MapFS{
				"src/app.ts":             &fstest.MapFile{Data: []byte("export const getUsers = () => {};\n")},
				"src/services/users.ts":  &fstest.MapFile{Data: []byte("export const noop = 1;\n")},
				"src/services/orders.ts": &fstest.MapFile{Data: []byte("export const noop2 = 1;\n")},
			}
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), realisticSnapshot, testMeta(), sidecarOptsWithMode("reachability"), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"src/services"}})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(HaveLen(1), "expected exactly one bypass witness from the real depth-1 edge shape, got %+v", result.Witnesses)
			witness := result.Witnesses[0]
			Expect(witness.Source).To(Equal("file:src/app.ts#getUsers"))
			Expect(witness.Sink).To(Equal("(PrismaClient).findMany"))
			Expect(witness.RequiredLayer).To(Equal("service"))
			Expect(witness.Confidence).To(Equal(projectmodel.LayerBypassConfidenceHigh))

			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("the sidecar reports incomplete coverage alongside a resolvable bypass edge", func() {
		It("suppresses the witness rather than reporting a genuinely searched result", func() {
			result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsLayerBypassSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_gap"), requiredServiceLayer)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Witnesses).To(BeEmpty(), "an incomplete sidecar round trip must suppress every witness, even one that would otherwise resolve, got %+v", result.Witnesses)
			Expect(result.Coverage.Complete).To(BeFalse())
		})
	})
})
