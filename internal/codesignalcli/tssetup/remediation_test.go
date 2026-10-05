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

type sigTestPrepareCompilerRemediationOffersOnlyExecutablePrepare struct {
	code string
	got  string
	t    *testing.
		T
}

func (sigRecv *sigTestPrepareCompilerRemediationOffersOnlyExecutablePrepare) call() {

	if sigRecv.got == "" {
		sigRecv.t.
			Errorf("PrepareCompilerRemediation(%q, ...) = \"\", want a non-empty --prepare-compiler command", sigRecv.code)
	}
}

// TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps pins
// AC-SET-10's runtime-boundary installer suppression against every gap code
// projectreadiness.KnownGapCodes() currently defines, via a literal per-code expectation rather
// than an expression derived from production code: deriving "should offer a
// command" from projectreadiness.NextActionExecutable/projectreadiness.KnownGapCodes() itself (as
// PrepareCompilerRemediation does) would make the assertion tautological and
// unable to catch a mutation to either.
func TestPrepareCompilerRemediationOffersOnlyExecutablePrepareCompilerGaps(t *testing.T) {
	wantCommand := map[string]bool{
		projectreadiness.GapUnsupportedRepositoryShape:        false,
		projectreadiness.GapNodeMissing:                       false,
		projectreadiness.GapNodeUnsupported:                   false,
		projectreadiness.GapNodeUnverifiable:                  false,
		projectreadiness.GapTypescriptCompilerMissing:         true,
		projectreadiness.GapTypescriptVersionMismatch:         true,
		projectreadiness.GapTypescriptVersionConflict:         true,
		projectreadiness.GapPackageManagerAmbiguous:           false,
		projectreadiness.GapPackageManagerConfigUnverifiable:  false,
		projectreadiness.GapPackageManagerVersionUnverifiable: false,
		projectreadiness.GapPackageManagerVersionUnsupported:  false,
		projectreadiness.GapPolicyMissing:                     false,
		projectreadiness.GapPolicyInvalid:                     false,
	}

	if len(wantCommand) != len(projectreadiness.KnownGapCodes()) {
		t.Fatalf("wantCommand has %d entries, gapCodeTable has %d: add the missing gap code to wantCommand with an explicit executable/non-executable decision", len(wantCommand), len(projectreadiness.KnownGapCodes()))
	}
	for _, code := range projectreadiness.KnownGapCodes() {
		if _, ok := wantCommand[code]; !ok {
			t.Fatalf("gapCodeTable defines %q but wantCommand does not: add an explicit executable/non-executable decision for it", code)
		}
	}

	for code, wantExecutable := range wantCommand {
		got := PrepareCompilerRemediation(code, "project.json")
		if wantExecutable {
			(&sigTestPrepareCompilerRemediationOffersOnlyExecutablePrepare{code: code, got: got, t: t}).call()

			continue
		}
		if got != "" {
			t.Errorf("PrepareCompilerRemediation(%q, ...) = %q, want empty: AC-SET-10 forbids offering a command for a non-prepare-compiler gap", code, got)
		}
	}
}
