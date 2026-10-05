package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestAlsoFailingGapLinesReportsEveryGapWithoutClaimingItSurvives pins
// AC-SET-13's report-all-gaps clause: every non-policy gap readiness.Gaps
// carries is reported, in readiness's own order, and the line names the
// --check-project rerun with no hedge about whether the gap outlives the
// policy failure. Since R1, checkProjectShape and pkgmanager.Check
// report not_checked instead of guessing from the worktree root when the
// policy has failed, so nothing left in readiness.Gaps here is an artifact
// of that guess to hedge about in the first place -- a package-manager
// version is probed from the manager binary and never depends on roots,
// and a shape finding that would have needed the guess is absent from
// Gaps entirely rather than present and uncertain.
func TestAlsoFailingGapLinesReportsEveryGapWithoutClaimingItSurvives(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing}},
		Gaps: []projectreadiness.Gap{
			{Code: projectreadiness.GapPolicyMissing},
			{Code: projectreadiness.GapNodeUnsupported},
			{Code: projectreadiness.GapTypescriptCompilerMissing},
			{Code: projectreadiness.GapPackageManagerVersionUnsupported},
		},
	}
	got := AlsoFailingGapLines(readiness, "project.json")
	want := []string{projectreadiness.GapNodeUnsupported, projectreadiness.GapTypescriptCompilerMissing, projectreadiness.GapPackageManagerVersionUnsupported}
	if len(got) != len(want) {
		t.Fatalf("AlsoFailingGapLines() = %q, want one line for each of %q", got, want)
	}
	for i, code := range want {
		if !strings.HasPrefix(got[i], code+":") {
			t.Fatalf("AlsoFailingGapLines()[%d] = %q, want the %q gap in readiness's own frozen order", i, got[i], code)
		}
		if strings.Contains(got[i], "once the policy above is authored and committed") || strings.Contains(got[i], "checked without the policy's selected roots") {
			t.Fatalf("AlsoFailingGapLines()[%d] = %q: the line must not hedge about whether the gap outlives the policy failure -- R1 already withholds any gap that guess would have produced", i, got[i])
		}
		if !strings.Contains(got[i], "run coach codesignal --baseline --check-project") {
			t.Fatalf("AlsoFailingGapLines()[%d] = %q, want it to name the --check-project rerun", i, got[i])
		}
	}

	t.Run("a policy-only gap list prints nothing extra", func(t *testing.T) {
		body_projectTsPreflightPart2Test_aPolicyOnlyGapListPrintsNothingExtra_48(t)
	})
	t.Run("nil readiness prints nothing extra", func(t *testing.T) {
		body_projectTsPreflightPart2Test_nilReadinessPrintsNothingExtra_53(t)
	})
}

// TestRunCompilerSetupOfferReportsNoChoicesOffered pins the fail-fast cases
// where RunCompilerSetupOffer must never print a single byte of prompt: a
// nil readiness snapshot, a readiness result whose compiler check is not
// actually failing, and a failing compiler check for which
// AvailableSetupChoices offers only its always-appended cancel entry (no
// package manager, no verified mise scope).
func TestRunCompilerSetupOfferReportsNoChoicesOffered(t *testing.T) {
	cases := map[string]*projectreadiness.Result{
		"nil readiness":          nil,
		"compiler check passing": {Checks: projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Pass}, Compiler: projectreadiness.Check{State: projectreadiness.Pass}}},
		"compiler check failing but no choice is offerable": {
			Checks: projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Pass}, Compiler: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing}},
		},
	}
	for name, readiness := range cases {
		t.Run(name, func(t *testing.T) {
			body_projectTsPreflightPart2Test_75(t, readiness)
		})
	}
}
