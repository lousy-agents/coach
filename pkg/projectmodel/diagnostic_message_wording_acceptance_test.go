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
})
