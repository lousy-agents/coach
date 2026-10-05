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

func TestAppendedRemediationLine(t *testing.T) {
	const line = "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"

	t.Run("without a controlling terminal prints the remediation line", func(t *testing.T) {
		withoutControllingTerminalPrintsRemediationLine(t, line)
	})
	t.Run("a TypeScript controlling terminal withholds the line because the interactive offer owns it", func(t *testing.T) {
		aTypeScriptControllingTerminalWithholdsLineBecauseInteractive(t, line)
	})
	t.Run("a Go controlling terminal still prints the line because Go has no interactive offer", func(t *testing.T) {
		aGoControllingTerminalStillPrintsLineBecause(t, line)
	})
}

func TestSuggestProjectConfigRemediation(t *testing.T) {
	t.Run("TypeScript names the guided-authoring invocation plus the no-terminal path", typeScriptNamesGuidedAuthoringInvocationPlusNoTerminal)
	t.Run("Go is offered only the non-interactive suggest command", goOfferedOnlyNonInteractiveSuggestCommand)
}

func withoutControllingTerminalPrintsRemediationLine(t *testing.T, line string) {
	if got := AppendedRemediationLine(false, "typescript", line); got != line {
		t.Fatalf("AppendedRemediationLine(false, %q, line) = %q, want %q", "typescript", got, line)
	}
}

func aTypeScriptControllingTerminalWithholdsLineBecauseInteractive(t *testing.T, line string) {
	if got := AppendedRemediationLine(true, "typescript", line); got != "" {
		t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want empty: the interactive setup offer owns the controlling-terminal case for typescript", "typescript", got)
	}
}

func aGoControllingTerminalStillPrintsLineBecause(t *testing.T, line string) {
	if got := AppendedRemediationLine(true, "go", line); got != line {
		t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want %q: go has no interactive setup offer to own the controlling-terminal case", "go", got, line)
	}
}

func typeScriptNamesGuidedAuthoringInvocationPlusNoTerminal(t *testing.T) {
	ts := SuggestProjectConfigRemediation("typescript")
	if !strings.Contains(ts, "coach codesignal --baseline --suggest-project-config --project-language typescript") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to still name the guided-authoring invocation", "typescript", ts)
	}
	if !strings.Contains(ts, "requires a controlling terminal") || !strings.Contains(ts, "draft the schema-1 project-config document yourself") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to state the terminal requirement and the no-terminal path, since this line is printed only when no controlling terminal is available", "typescript", ts)
	}
}

func goOfferedOnlyNonInteractiveSuggestCommand(t *testing.T) {
	if got, want := SuggestProjectConfigRemediation("go"), "coach codesignal --baseline --suggest-project-config"; got != want {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want %q: a go scan must never be offered the TypeScript-only guided-authoring command (AC-2)", "go", got, want)
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
