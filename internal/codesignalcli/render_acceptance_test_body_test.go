package codesignalcli

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/gomega"
)

func body_renderAcceptanceTest_rendersTheProjectScopeBlockBEFOREAnyFindingsSect_68(report *codesignal.Report) {
	text := RenderText(report)

	scopeIdx := strings.Index(text, "Project scope:")
	noFindingsIdx := strings.Index(text, "No active CodeSignal findings")

	Expect(scopeIdx).To(BeNumerically(">=", 0), "Project scope: block must be present; text=\n%s", text)
	if noFindingsIdx >= 0 {
		Expect(scopeIdx).To(BeNumerically("<", noFindingsIdx), "Project scope: must precede no-findings verdict")
	}
	Expect(text).To(ContainSubstring("pattern_set: ts-source-sink-registry@1"))
	Expect(text).To(ContainSubstring("matched_layers: handlers"))
	Expect(text).To(ContainSubstring("unmatched_layers: db"))
}
