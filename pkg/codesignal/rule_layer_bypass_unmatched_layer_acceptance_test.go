package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

var _ = DescribeTable("the layer-bypass evaluators carry the cause of an unmatched required layer into the report's diagnostics",
	func(evaluate func(domain.LayerBypassResult, string, string, string) ([]codesignal.ProjectChange, []codesignal.Diagnostic)) {
		const message = `required_layer "service" matched no package`
		unmatched := domain.LayerBypassResult{Coverage: domain.Coverage{Diagnostics: []domain.Diagnostic{
			{Code: domain.DiagLayerBypassAmbiguousLayer},
			{Code: domain.DiagLayerBypassRequiredLayerUnmatched, Message: message},
		}}}
		unrelated := domain.LayerBypassResult{Coverage: domain.Coverage{Diagnostics: []domain.Diagnostic{{Code: domain.DiagLayerBypassAmbiguousLayer}}}}

		_, withCause := evaluate(unmatched, "v1", "backend", "digest")
		_, withoutCause := evaluate(unrelated, "v1", "backend", "digest")

		Expect(withCause).To(ContainElement(And(HaveField("Kind", "project_layer_bypass_required_layer_unmatched"), HaveField("Message", message))))
		for _, diagnostic := range withoutCause {
			Expect(diagnostic.Kind).NotTo(Equal("project_layer_bypass_required_layer_unmatched"))
		}
	},
	Entry("Go", codesignal.EvaluateGoLayerBypass),
	Entry("TypeScript", codesignal.EvaluateTypeScriptLayerBypass),
)
