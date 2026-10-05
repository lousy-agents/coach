package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func body_projectTsPreflightTest_aShapedFailingCompilerIsReportedFromGaps_56(t *testing.T) {
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

func body_projectTsPreflightTest_aPassingCompilerWithNoGapsReportsNothing_70(t *testing.T) {
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

func body_projectTsPreflightTest_anUnsupportedRepositoryShapeStillReportsEveryGap_82(t *testing.T) {
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

func body_projectTsPreflightTest_aFailingCheckAbsentFromGapsIsNotReported_96(t *testing.T) {
	compilerFailingButNotInGaps := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Compiler: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing},
		},
	}
	if got := AlsoFailingGapLines(compilerFailingButNotInGaps, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(failing check absent from gaps[]) = %q, want none", got)
	}
}
