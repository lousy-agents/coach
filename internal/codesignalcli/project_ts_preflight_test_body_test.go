package codesignalcli

import (
	"strings"
	"testing"
)

func body_projectTsPreflightTest_aShapedFailingCompilerIsReportedFromGaps_56(t *testing.T) {
	failingCompilerShaped := &ReadinessResult{
		Checks: ReadinessChecks{
			ProjectShape: ReadinessCheck{State: ReadinessPass},
			Compiler:     ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
		},
		Gaps: []ReadinessGap{{Code: GapTypescriptCompilerMissing}},
	}
	want := "typescript_compiler_missing: also failing, run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"
	if got := AlsoFailingGapLines(failingCompilerShaped, "project.json"); len(got) != 1 || got[0] != want {
		t.Fatalf("AlsoFailingGapLines(shaped) = %q, want exactly [%q]", got, want)
	}
}

func body_projectTsPreflightTest_aPassingCompilerWithNoGapsReportsNothing_70(t *testing.T) {
	passingCompiler := &ReadinessResult{
		Checks: ReadinessChecks{
			ProjectShape: ReadinessCheck{State: ReadinessPass},
			Compiler:     ReadinessCheck{State: ReadinessPass},
		},
	}
	if got := AlsoFailingGapLines(passingCompiler, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(passing compiler) = %q, want none", got)
	}
}

func body_projectTsPreflightTest_anUnsupportedRepositoryShapeStillReportsEveryGap_82(t *testing.T) {
	notShapedButGapped := &ReadinessResult{
		Checks: ReadinessChecks{
			ProjectShape: ReadinessCheck{State: ReadinessFail, Code: GapUnsupportedRepositoryShape},
			Compiler:     ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
		},
		Gaps: []ReadinessGap{{Code: GapUnsupportedRepositoryShape}, {Code: GapTypescriptCompilerMissing}},
	}
	got := AlsoFailingGapLines(notShapedButGapped, "project.json")
	if len(got) != 2 || !strings.HasPrefix(got[0], GapUnsupportedRepositoryShape+":") || !strings.HasPrefix(got[1], GapTypescriptCompilerMissing+":") {
		t.Fatalf("AlsoFailingGapLines(unsupported repository shape, still gapped) = %q, want both gaps in readiness's own order", got)
	}
}

func body_projectTsPreflightTest_aFailingCheckAbsentFromGapsIsNotReported_96(t *testing.T) {
	compilerFailingButNotInGaps := &ReadinessResult{
		Checks: ReadinessChecks{
			Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
		},
	}
	if got := AlsoFailingGapLines(compilerFailingButNotInGaps, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(failing check absent from gaps[]) = %q, want none", got)
	}
}
