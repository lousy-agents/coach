package codesignalcli

import (
	"testing"
)

// TestRunCompilerSetupOfferNeverOpensForARuntimeBoundaryGap pins AC-13/
// AC-SET-10's interactive-path gate: RunCompilerSetupOffer must withhold the
// offer -- printing nothing at all -- for any gapCode whose next action is
// not the executable prepare-compiler kind, even when the readiness snapshot
// passed alongside it independently offers a genuine, installable menu
// entry. gapCode and readiness.Checks.Compiler.Code are two independent
// CheckProjectReadiness reads (project_readiness.go resolves Node and the
// compiler separately), so a runtime-boundary gap can coexist with an
// installable compiler menu; this is the fixture shape that previously drove
// the interactive menu open for node_missing/node_unsupported. A literal
// per-code expectation, not one derived from gapCodeIsExecutablePrepareCompiler
// itself, so a mutation to either cannot pass by construction.
func TestRunCompilerSetupOfferNeverOpensForARuntimeBoundaryGap(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}

	wantOffered := map[string]bool{
		GapNodeMissing:               false,
		GapNodeUnsupported:           false,
		GapNodeUnverifiable:          false,
		GapTypescriptCompilerMissing: true,
	}

	for gapCode, offered := range wantOffered {
		t.Run(gapCode, func(t *testing.T) {
			body_projectTsPreflightPart3Test_39(t, readiness, gapCode, offered)
		})
	}
}

func TestPrepareCompilerRemediation(t *testing.T) {
	if got, want := PrepareCompilerRemediation(GapTypescriptCompilerMissing, "project.json"), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", GapTypescriptCompilerMissing, "project.json", got, want)
	}
	if got, want := PrepareCompilerRemediation(GapTypescriptVersionMismatch, ""), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, \"\") = %q, want %q", GapTypescriptVersionMismatch, got, want)
	}
	if got, want := PrepareCompilerRemediation(GapTypescriptVersionConflict, "project.json"), "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript --project-config project.json"; got != want {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want %q", GapTypescriptVersionConflict, "project.json", got, want)
	}
	if got := PrepareCompilerRemediation(GapNodeMissing, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", GapNodeMissing, "project.json", got)
	}
	if got := PrepareCompilerRemediation(GapNodeUnsupported, "project.json"); got != "" {
		t.Fatalf("PrepareCompilerRemediation(%q, %q) = %q, want empty: Coach has no executable remediation for a runtime-boundary gap", GapNodeUnsupported, "project.json", got)
	}
}
