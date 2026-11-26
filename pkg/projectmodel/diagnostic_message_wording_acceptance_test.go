package projectmodel_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func defaultMessageOf(code string) string {
	coverage := domain.CanonicalCoverage(domain.Coverage{Diagnostics: []domain.Diagnostic{{Code: code}}})
	return coverage.Diagnostics[0].Message
}

var _ = Describe("the wording of a layer-bypass diagnostic is true and actionable (#894)", func() {
	It("tells a layer-bypass budget message's reader what was not searched and how to act", func() {
		message := defaultMessageOf(domain.DiagLayerBypassBudgetExceeded)

		Expect(message).To(ContainSubstring("did not run to completion"))
		Expect(message).To(ContainSubstring("does not mean there is no bypass"))
		Expect(message).To(ContainSubstring("narrow the policy's roots"))
	})

	It("says the required layer matches nothing in the analysed program, not only no function in a call graph", func() {
		message := defaultMessageOf(domain.DiagLayerBypassAmbiguousLayer)

		Expect(message).To(ContainSubstring("required_layer"))
		Expect(message).To(ContainSubstring("matches nothing in the analysed program"))
		Expect(message).NotTo(ContainSubstring("call graph"))
	})

	It("gives a TypeScript project's unmatched required layer the same program-wide wording", func() {
		result, err := projectmodel.BuildTypeScriptLayerBypass(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("layer_bypass_direct"), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"nonexistent_service_dir"}})
		Expect(err).NotTo(HaveOccurred())

		for _, diagnostic := range result.Coverage.Diagnostics {
			if diagnostic.Code == projectmodel.DiagLayerBypassAmbiguousLayer {
				Expect(diagnostic.Message).To(ContainSubstring("matches nothing in the analysed program"))
				Expect(diagnostic.Message).NotTo(ContainSubstring("call graph"))
				return
			}
		}
		Fail("the TypeScript scenario reports no project_layer_bypass_ambiguous_layer diagnostic")
	})
})
