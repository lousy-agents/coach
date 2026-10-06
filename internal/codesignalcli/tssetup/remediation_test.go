package tssetup

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestPrepareCompilerRemediation(t *testing.T) {
	if got, want := PrepareCompilerRemediation(projectreadiness.GapTypescriptCompilerMissing, "project.json"), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", projectreadiness.GapTypescriptCompilerMissing, "project.json", got, want)
	}
	if got, want := PrepareCompilerRemediation(projectreadiness.GapTypescriptVersionMismatch, ""), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, \"\") = %q, want %q", projectreadiness.GapTypescriptVersionMismatch, got, want)
	}
	if got, want := PrepareCompilerRemediation(projectreadiness.GapTypescriptVersionConflict, "project.json"), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", projectreadiness.GapTypescriptVersionConflict, "project.json", got, want)
	}
	if got := PrepareCompilerRemediation(projectreadiness.GapNodeMissing, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", projectreadiness.GapNodeMissing, "project.json", got)
	}
	if got := PrepareCompilerRemediation(projectreadiness.GapNodeUnsupported, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", projectreadiness.GapNodeUnsupported, "project.json", got)
	}
}

// TestPrepareCompilerRemediationWithReadinessWithholdsADeadEndCommand pins
// O2: even for a gap code whose next action is the executable
// prepare-compiler kind, the command must be withheld unless the attached
// readiness snapshot offers a choice --prepare-compiler itself can execute.
// That flag runs mise scopes only (filterMiseChoiceKinds), so a menu whose
// sole executable entry is project_package is as much a dead end as an empty
// one: the named command would open, discard the only offered choice, and
// exit 0 reporting it had nothing to set up while the compiler is still
// missing.
func TestPrepareCompilerRemediationWithReadinessWithholdsADeadEndCommand(t *testing.T) {
	nothingOffered := &projectreadiness.Result{
		Checks: projectreadiness.Checks{Compiler: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing}},
	}
	if got := PrepareCompilerRemediationWithReadiness(projectreadiness.GapTypescriptCompilerMissing, "project.json", nothingOffered); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(dead-end menu) = %q, want empty", got)
	}

	projectPackageOnly := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
	if got := PrepareCompilerRemediationWithReadiness(projectreadiness.GapTypescriptCompilerMissing, "project.json", projectPackageOnly); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(project_package-only menu) = %q, want empty: --prepare-compiler runs mise scopes only, so it would exit 0 having set nothing up", got)
	}

	miseOffered := &projectreadiness.Result{
		Checks:      projectreadiness.Checks{Compiler: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing}},
		MiseChoices: []projectreadiness.MiseChoice{{Kind: tstoolchain.OriginMiseProject, Verified: true}},
	}
	want := "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"
	if got := PrepareCompilerRemediationWithReadiness(projectreadiness.GapTypescriptCompilerMissing, "project.json", miseOffered); got != want {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(verified mise scope) = %q, want %q", got, want)
	}

	if got := PrepareCompilerRemediationWithReadiness(projectreadiness.GapNodeMissing, "project.json", miseOffered); got != "" {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(%q, ...) = %q, want empty: a runtime-boundary gap code must still be withheld regardless of readiness", projectreadiness.GapNodeMissing, got)
	}

	if got, want := PrepareCompilerRemediationWithReadiness(projectreadiness.GapTypescriptCompilerMissing, "project.json", nil), PrepareCompilerRemediation(projectreadiness.GapTypescriptCompilerMissing, "project.json"); got != want {
		t.Fatalf("PrepareCompilerRemediationWithReadiness(nil readiness) = %q, want the plain PrepareCompilerRemediation fallback %q", got, want)
	}
}
