package projectmodel_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("the Go layer-bypass search names why a required layer could not be applied", func() {
	build := func(fixture string, layer projectmodel.BypassLayer) projectmodel.LayerBypassResult {
		result, err := projectmodel.BuildGoLayerBypass(context.Background(), os.DirFS(fixture), projectmodel.LayerBypassOptions{RequiredLayer: layer})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("adds the unmatched-layer diagnostic, naming the layer, when no package sits under its prefixes", func() {
		result := build("testdata/go_layer_bypass_ambiguous", projectmodel.BypassLayer{Name: "service", Prefixes: []string{"pkg/service"}})

		Expect(result.Coverage.Diagnostics).To(ContainElement(And(
			HaveField("Code", projectmodel.DiagLayerBypassRequiredLayerUnmatched),
			HaveField("Message", ContainSubstring(`required_layer "service" matched no package`)),
		)))
	})

	It("does not add it when a package sits under the prefixes but declares no function, though the layer stays ambiguous", func() {
		result := build("testdata/go_layer_bypass_types_only", projectmodel.BypassLayer{Name: "service", Prefixes: []string{"pkg/service"}})

		Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassAmbiguousLayer)).To(BeTrue(), "got %+v", result.Coverage.Diagnostics)
		Expect(hasLayerBypassDiagnostic(result.Coverage.Diagnostics, projectmodel.DiagLayerBypassRequiredLayerUnmatched)).To(BeFalse(), "got %+v", result.Coverage.Diagnostics)
	})
})
