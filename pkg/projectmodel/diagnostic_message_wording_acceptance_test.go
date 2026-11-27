package projectmodel_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func defaultMessageOf(code string) string {
	coverage := domain.CanonicalCoverage(domain.Coverage{Diagnostics: []domain.Diagnostic{{Code: code}}})
	return coverage.Diagnostics[0].Message
}

func ambiguousLayerMessage(diagnostics []projectmodel.Diagnostic) string {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == projectmodel.DiagLayerBypassAmbiguousLayer {
			return diagnostic.Message
		}
	}
	Fail("the scenario reports no project_layer_bypass_ambiguous_layer diagnostic")
	return ""
}

func expectTrueForEveryCause(message string) {
	Expect(message).To(ContainSubstring("required_layer"))
	Expect(message).To(ContainSubstring("no function or file the layer-bypass search follows lies under its prefixes"))
	Expect(message).To(ContainSubstring("a layer whose packages declare only types has none"))
	Expect(message).NotTo(ContainSubstring("matches nothing"), "a layer that holds only types does match its package")
	Expect(message).NotTo(ContainSubstring("call graph"))
}

func expectSecondCauseRemedy(message string) {
	Expect(message).To(ContainSubstring("add what the search reached to a layer"))
	Expect(message).To(ContainSubstring("widen a layer's prefixes to cover it"))
}

var _ = Describe("the wording of a layer-bypass diagnostic is true and actionable (#894)", func() {
	goLayerBypass := func(fixture string) projectmodel.LayerBypassResult {
		result, err := projectmodel.BuildGoLayerBypass(context.Background(), os.DirFS(fixture), projectmodel.LayerBypassOptions{RequiredLayer: projectmodel.BypassLayer{Name: "service", Prefixes: []string{"pkg/service"}}})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("tells a layer-bypass budget message's reader what was not searched and how to act", func() {
		message := defaultMessageOf(domain.DiagLayerBypassBudgetExceeded)

		Expect(message).To(ContainSubstring("did not run to completion"))
		Expect(message).To(ContainSubstring("does not mean there is no bypass"))
		Expect(message).To(ContainSubstring("narrow the policy's roots"))
	})

	It("words the ambiguous-layer message so it is true for every cause", func() {
		expectTrueForEveryCause(defaultMessageOf(domain.DiagLayerBypassAmbiguousLayer))
	})

	It("gives a Go layer with no package the same wording", func() {
		expectTrueForEveryCause(ambiguousLayerMessage(goLayerBypass("testdata/go_layer_bypass_ambiguous").Coverage.Diagnostics))
	})

	It("gives a Go layer whose package declares only types the same wording, and does not say it matches nothing", func() {
		expectTrueForEveryCause(ambiguousLayerMessage(goLayerBypass("testdata/go_layer_bypass_types_only").Coverage.Diagnostics))
	})

	It("gives a TypeScript project's unmatched required layer the same wording", func() {
		result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}})
		Expect(err).NotTo(HaveOccurred())

		expectTrueForEveryCause(ambiguousLayerMessage(result.Coverage.Diagnostics))
	})

	DescribeTable("the ambiguous-layer message names a remedy for each cause it gives (#913)",
		func(remedy string) {
			Expect(defaultMessageOf(domain.DiagLayerBypassAmbiguousLayer)).To(ContainSubstring(remedy))
		},
		Entry("for a layer with nothing under its prefixes: check the prefixes", "check the layer's prefixes"),
		Entry("for a layer with nothing under its prefixes: add code under them", "add code under them"),
		Entry("for something outside every layer: add it to a layer", "add what the search reached to a layer"),
		Entry("for something outside every layer: widen a layer's prefixes", "widen a layer's prefixes to cover it"),
	)

	It("no longer gives the one action that does not fit the second cause", func() {
		Expect(defaultMessageOf(domain.DiagLayerBypassAmbiguousLayer)).NotTo(ContainSubstring("or add code to the layer"))
	})

	It("gives a Go layer with no package the second cause's remedy too", func() {
		expectSecondCauseRemedy(ambiguousLayerMessage(goLayerBypass("testdata/go_layer_bypass_ambiguous").Coverage.Diagnostics))
	})

	It("gives a TypeScript project's unmatched required layer the second cause's remedy too", func() {
		result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}})
		Expect(err).NotTo(HaveOccurred())

		expectSecondCauseRemedy(ambiguousLayerMessage(result.Coverage.Diagnostics))
	})

	It("gives a TypeScript project whose search reached a node outside every layer the remedy for that cause", func() {
		model := projectmodel.Model{
			Files:             []projectmodel.File{{Path: "service/inventory.ts"}, {Path: "src/handler.ts"}},
			CallFacts:         []projectmodel.CallFact{{From: "file:src/handler.ts#handle", To: "a node whose id names no file"}, {From: "a node whose id names no file", To: "db.query"}},
			ReachabilityFacts: []projectmodel.ReachabilityFact{{Source: "file:src/handler.ts#handle", Sink: "db.query"}},
			Coverage:          projectmodel.Coverage{Complete: true},
		}

		result := projectmodel.BuildTypeScriptLayerBypassFromModel(context.Background(), model, projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}})

		expectSecondCauseRemedy(ambiguousLayerMessage(result.Coverage.Diagnostics))
	})
})
