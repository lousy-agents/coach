package tssetup

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

	t.Run("a policy-only gap list prints nothing extra", aPolicyOnlyGapListPrintsNothingExtra)
	t.Run("nil readiness prints nothing extra", nilReadinessPrintsNothingExtra)
}

func aPolicyOnlyGapListPrintsNothingExtra(t *testing.T) {
	if got := AlsoFailingGapLines(&projectreadiness.Result{Gaps: []projectreadiness.Gap{{Code: projectreadiness.GapPolicyMissing}}}, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(policy gap only) = %q, want none: the policy failure is already printed as the scan's own message", got)
	}
}

func nilReadinessPrintsNothingExtra(t *testing.T) {
	if got := AlsoFailingGapLines(nil, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(nil readiness) = %q, want none", got)
	}
}

func TestAlsoFailingGapLinesFollowsReadinessGaps(t *testing.T) {
	t.Run("a shaped failing compiler is reported from gaps", aShapedFailingCompilerReportedFromGaps)

	t.Run("a passing compiler with no gaps reports nothing", aPassingCompilerNoGapsReportsNothing)

	t.Run("an unsupported repository shape still reports every gap readiness names", anUnsupportedRepositoryShapeStillReportsEveryGap)

	t.Run("a failing check absent from gaps is not reported", aFailingCheckAbsentFromGapsNotReported)
}

func aShapedFailingCompilerReportedFromGaps(t *testing.T) {
	failingCompilerShaped := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			ProjectShape: projectreadiness.Check{State: projectreadiness.Pass},
			Compiler:     projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		},
		Gaps: []projectreadiness.Gap{{Code: projectreadiness.GapTypescriptCompilerMissing}},
	}
	want := "typescript_compiler_missing: also failing, run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"
	if got := AlsoFailingGapLines(failingCompilerShaped, "project.json"); len(got) != 1 || got[0] != want {
		t.Fatalf("AlsoFailingGapLines(shaped) = %q, want exactly [%q]", got, want)
	}
}

func aPassingCompilerNoGapsReportsNothing(t *testing.T) {
	passingCompiler := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			ProjectShape: projectreadiness.Check{State: projectreadiness.Pass},
			Compiler:     projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
	if got := AlsoFailingGapLines(passingCompiler, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(passing compiler) = %q, want none", got)
	}
}

func anUnsupportedRepositoryShapeStillReportsEveryGap(t *testing.T) {
	notShapedButGapped := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			ProjectShape: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapUnsupportedRepositoryShape},
			Compiler:     projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		},
		Gaps: []projectreadiness.Gap{{Code: projectreadiness.GapUnsupportedRepositoryShape}, {Code: projectreadiness.GapTypescriptCompilerMissing}},
	}
	got := AlsoFailingGapLines(notShapedButGapped, "project.json")
	if len(got) != 2 || !strings.HasPrefix(got[0], projectreadiness.GapUnsupportedRepositoryShape+":") || !strings.HasPrefix(got[1], projectreadiness.GapTypescriptCompilerMissing+":") {
		t.Fatalf("AlsoFailingGapLines(unsupported repository shape, still gapped) = %q, want both gaps in readiness's own order", got)
	}
}

func aFailingCheckAbsentFromGapsNotReported(t *testing.T) {
	compilerFailingButNotInGaps := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Compiler: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		},
	}
	if got := AlsoFailingGapLines(compilerFailingButNotInGaps, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(failing check absent from gaps[]) = %q, want none", got)
	}
}
