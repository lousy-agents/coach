package tssetup

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func failureWithOtherGaps(cause projectreadiness.PolicyCause) *ProjectConfigErrorWithReadiness {
	return &ProjectConfigErrorWithReadiness{
		ConfigError: &projectconfig.ConfigError{PolicyCause: cause, Message: "the policy is refused"},
		Readiness: &projectreadiness.Result{Gaps: []projectreadiness.Gap{
			{Code: projectreadiness.GapPolicyInvalid},
			{Code: projectreadiness.GapNodeUnsupported},
			{Code: projectreadiness.GapTypescriptCompilerMissing},
		}},
		Policy: policyref.Named("CFG/p.json"),
	}
}

// A policy refused for the case of its name is refused again by any rerun that names it as typed, so the lines
// for the other gaps still say what is failing but print no command that repeats the spelling just refused.
func TestTheOtherGapsOfAPolicyRefusedForItsSpellingNameNoRerunOfThatSpelling(t *testing.T) {
	got := failureWithOtherGaps(projectreadiness.CauseNameCaseDiffers).AlsoFailingLines()

	want := []string{projectreadiness.GapNodeUnsupported + ": also failing", projectreadiness.GapTypescriptCompilerMissing + ": also failing"}
	if len(got) != len(want) {
		t.Fatalf("AlsoFailingLines() = %q; want one line for each of %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] || strings.Contains(got[i], "CFG/p.json") || strings.Contains(got[i], "--project-config") {
			t.Errorf("AlsoFailingLines()[%d] = %q; want %q, with no command", i, got[i], want[i])
		}
	}
}

func TestTheOtherGapsOfAnyOtherPolicyFailureStillNameTheRerun(t *testing.T) {
	got := failureWithOtherGaps(projectreadiness.CauseNotCommitted).AlsoFailingLines()

	if len(got) != 2 || !strings.Contains(got[0], "run coach codesignal --baseline --check-project") || !strings.Contains(got[0], "CFG/p.json") {
		t.Errorf("AlsoFailingLines() = %q; want each gap with the rerun that names the policy", got)
	}
}
