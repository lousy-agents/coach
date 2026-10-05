package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// TestRunCompilerSetupOfferNeverOpensForARuntimeBoundaryGap pins AC-13/
// AC-SET-10's interactive-path gate: RunCompilerSetupOffer must withhold the
// offer -- printing nothing at all -- for any gapCode whose next action is
// not the executable prepare-compiler kind, even when the readiness snapshot
// passed alongside it independently offers a genuine, installable menu
// entry. gapCode and readiness.Checks.Compiler.Code are two independent
// projectcheck.Run reads (project_readiness.go resolves Node and the
// compiler separately), so a runtime-boundary gap can coexist with an
// installable compiler menu; this is the fixture shape that previously drove
// the interactive menu open for node_missing/node_unsupported. A literal
// per-code expectation, not one derived from gapCodeIsExecutablePrepareCompiler
// itself, so a mutation to either cannot pass by construction.
func TestRunCompilerSetupOfferNeverOpensForARuntimeBoundaryGap(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Policy:         projectreadiness.Check{State: projectreadiness.Pass},
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}

	wantOffered := map[string]bool{
		projectreadiness.GapNodeMissing:               false,
		projectreadiness.GapNodeUnsupported:           false,
		projectreadiness.GapNodeUnverifiable:          false,
		projectreadiness.GapTypescriptCompilerMissing: true,
	}

	for gapCode, offered := range wantOffered {
		t.Run(gapCode, func(t *testing.T) {
			body_projectTsPreflightPart3Test_39(t, readiness, gapCode, offered)
		})
	}
}

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
