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
})
