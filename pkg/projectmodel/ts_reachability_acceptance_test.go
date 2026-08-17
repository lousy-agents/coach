package projectmodel_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("BuildTypeScriptReachability", func() {
	When("a TS route handler has a resolved call path to a pinned query-shaped sink", func() {
		It("produces a possible_call_reachability ReachabilityFact with structured path, resolved-direct confidence, and the TS algorithm identity", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability"))
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Algorithm).To(Equal("ts-source-sink-registry@1"))
			Expect(result.Algorithm).NotTo(Equal(projectmodel.ReachabilityAlgorithm),
				"expected the TS traversal's own algorithm identity, distinct from Go's go-source-sink-registry@1")

			Expect(result.Facts).To(HaveLen(1), "expected exactly one reachability fact, got %+v", result.Facts)
			fact := result.Facts[0]
			Expect(fact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
			Expect(fact.Confidence).To(Equal(projectmodel.ReachabilityConfidenceResolvedDirect))
			Expect(fact.Source).To(Equal("file:src/app.ts#getUsers"))
			Expect(fact.Sink).To(Equal("(PrismaClient).findMany"))
			Expect(fact.Path).To(Equal([]projectmodel.ReachabilityStep{
				{NodeID: "file:src/app.ts#getUsers"},
				{NodeID: "(PrismaClient).findMany"},
			}))
			Expect(fact.AlgorithmVersion).To(Equal("ts-source-sink-registry@1"))

			Expect(result.Sources).To(Equal([]string{"file:src/app.ts#getUsers"}),
				"Sources on the TS path is derived from the resolved facts' own Source values, a narrower claim than Go's independently-identified handler set")
			Expect(result.Coverage.Complete).To(BeTrue())
		})
	})

	When("the sidecar reports several facts, two sharing one source and a second source out of sorted order", func() {
		It("returns Sources as the deduplicated, sorted set of fact Source values", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability_multi"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(HaveLen(3))
			Expect(result.Sources).To(Equal([]string{
				"file:src/app.ts#createUser",
				"file:src/app.ts#getUsers",
			}), "expected a deduplicated, sorted Sources set, got %v", result.Sources)
		})
	})

	When("the sidecar reports no reachability facts for this snapshot", func() {
		It("returns empty Facts and Sources without error, having achieved complete coverage", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("happy"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(BeEmpty())
			Expect(result.Sources).To(BeEmpty())
			Expect(result.Algorithm).To(Equal("ts-source-sink-registry@1"))
			Expect(result.Coverage.Complete).To(BeTrue(),
				"expected complete coverage so this case is distinguishable from the backend-unavailable case below, which also yields zero facts")
		})
	})

	When("the sidecar reports a coverage gap alongside a resolved fact", func() {
		It("still returns the resolved fact but with Coverage.Complete false", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability_gap"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(HaveLen(1))
			Expect(result.Coverage.Complete).To(BeFalse())
		})
	})

	When("the sidecar backend is unavailable", func() {
		It("returns empty Facts and Sources, incomplete coverage, and a project_backend_unavailable diagnostic instead of a Go error", func() {
			result, err := projectmodel.BuildTypeScriptReachability(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("crash"))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Facts).To(BeEmpty())
			Expect(result.Sources).To(BeEmpty())
			Expect(result.Coverage.Complete).To(BeFalse())
			_, ok := diagnosticWithCode(result.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", result.Coverage.Diagnostics)
		})
	})
})
