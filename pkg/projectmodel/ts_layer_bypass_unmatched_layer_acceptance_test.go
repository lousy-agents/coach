package projectmodel_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("the TypeScript layer-bypass search names why a required layer could not be applied", func() {
	build := func(layer projectmodel.BypassLayer) projectmodel.LayerBypassResult {
		result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsLayerBypassSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), layer)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("adds a diagnostic naming the required layer, next to the ambiguous-layer one, when its prefixes match no file", func() {
		result := build(projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}})

		Expect(result.Coverage.Diagnostics).To(ContainElement(And(
			HaveField("Code", projectmodel.DiagLayerBypassRequiredLayerUnmatched),
			HaveField("Message", And(ContainSubstring(`required_layer "service"`), ContainSubstring("matched no package"))),
		)))
		Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue())
	})

	It("does not add it when the required layer matches a file", func() {
		result := build(projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service/inventory.ts"}})

		Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassRequiredLayerUnmatched)).To(BeFalse(), "got %+v", result.Coverage.Diagnostics)
	})

	It("does not add it when the layer matches a file but a node on the path lies outside every layer, which is ambiguous for another reason", func() {
		model := projectmodel.Model{
			Files:             []projectmodel.File{{Path: "service/inventory.ts"}, {Path: "src/handler.ts"}},
			CallFacts:         []projectmodel.CallFact{{From: "file:src/handler.ts#handle", To: "a node whose id names no file"}, {From: "a node whose id names no file", To: "db.query"}},
			ReachabilityFacts: []projectmodel.ReachabilityFact{{Source: "file:src/handler.ts#handle", Sink: "db.query"}},
			Coverage:          projectmodel.Coverage{Complete: true},
		}

		result := projectmodel.BuildTypeScriptLayerBypassFromModel(context.Background(), model, projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}})

		Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue(), "got %+v", result.Coverage.Diagnostics)
		Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassRequiredLayerUnmatched)).To(BeFalse(), "got %+v", result.Coverage.Diagnostics)
	})
})
