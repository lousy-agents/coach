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

func expectSecondCauseNamedTruthfully(message string) {
	Expect(message).To(ContainSubstring("has no source position, so its layer could not be decided"))
	Expect(message).To(ContainSubstring("a limit of the analysis, not of the policy"))
	Expect(message).NotTo(ContainSubstring("outside every layer"), "no layer other than the required one is ever consulted")
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

	DescribeTable("the ambiguous-layer message names each cause for what the search tests and gives a remedy only where the policy has one (#913)",
		func(phrase string) {
			Expect(defaultMessageOf(domain.DiagLayerBypassAmbiguousLayer)).To(ContainSubstring(phrase))
		},
		Entry("first cause: check the prefixes", "check the layer's prefixes"),
		Entry("first cause: add code under them", "add code under them"),
		Entry("second cause: a function on the path has no source position", "a function on a path the search followed has no source position, so its layer could not be decided"),
		Entry("second cause: it is a limit of the analysis, not of the policy", "a limit of the analysis, not of the policy"),
		Entry("second cause: treat the run's result as incomplete", "treat this run's layer-bypass result as incomplete"),
	)

	DescribeTable("the ambiguous-layer message no longer says what the code does not test or offers a policy action that cannot clear the second cause (#913)",
		func(phrase string) {
			Expect(defaultMessageOf(domain.DiagLayerBypassAmbiguousLayer)).NotTo(ContainSubstring(phrase))
		},
		Entry("a node outside every layer: no layer other than the required one is consulted", "outside every layer"),
		Entry("add the reached package to a layer", "add what the search reached to a layer"),
		Entry("widen a layer's prefixes, which only moves the source inside the required layer", "widen a layer's prefixes"),
		Entry("the old action that fits only the first cause", "or add code to the layer"),
	)

	It("gives a Go layer with no package the same truthful second cause", func() {
		expectSecondCauseNamedTruthfully(ambiguousLayerMessage(goLayerBypass("testdata/go_layer_bypass_ambiguous").Coverage.Diagnostics))
	})

	It("gives a TypeScript project's unmatched required layer the same truthful second cause", func() {
		result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}})
		Expect(err).NotTo(HaveOccurred())

		expectSecondCauseNamedTruthfully(ambiguousLayerMessage(result.Coverage.Diagnostics))
	})

	It("gives a TypeScript project whose search followed a node id that names no file the second cause, worded for what triggered it", func() {
		model := projectmodel.Model{
			Files:             []projectmodel.File{{Path: "service/inventory.ts"}, {Path: "src/handler.ts"}},
			CallFacts:         []projectmodel.CallFact{{From: "file:src/handler.ts#handle", To: "a node whose id names no file"}, {From: "a node whose id names no file", To: "db.query"}},
			ReachabilityFacts: []projectmodel.ReachabilityFact{{Source: "file:src/handler.ts#handle", Sink: "db.query"}},
			Coverage:          projectmodel.Coverage{Complete: true},
		}

		result := projectmodel.BuildTypeScriptLayerBypassFromModel(context.Background(), model, projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}})

		expectSecondCauseNamedTruthfully(ambiguousLayerMessage(result.Coverage.Diagnostics))
	})
})
