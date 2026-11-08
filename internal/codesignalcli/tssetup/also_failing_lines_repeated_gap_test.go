package tssetup

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAGapReadinessReportsTwiceIsReportedOnce(t *testing.T) {
	twice := &projectreadiness.Result{Gaps: []projectreadiness.Gap{{Code: projectreadiness.GapNodeUnsupported}, {Code: projectreadiness.GapNodeUnsupported}}}

	if got := policyFailureBeside(twice).AlsoFailingLines(); len(got) != 1 {
		t.Fatalf("AlsoFailingLines() = %q, want one line for the gap that was listed twice", got)
	}
}
