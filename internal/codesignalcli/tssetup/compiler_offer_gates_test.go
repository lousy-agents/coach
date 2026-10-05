package tssetup

import (
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// TestRunCompilerSetupOfferReportsNoChoicesOffered pins the fail-fast cases
// where RunCompilerSetupOffer must never print a single byte of prompt: a
// nil readiness snapshot, a readiness result whose compiler check is not
// actually failing, and a failing compiler check for which
// AvailableChoices offers only its always-appended cancel entry (no
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

func body_projectTsPreflightPart2Test_75(t *testing.T, readiness *projectreadiness.Result) {
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", projectreadiness.GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)
	if !result.NoChoicesOffered {
		t.Fatalf("NoChoicesOffered = false, want true: %+v", result)
	}
	if result.Cancelled || result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output at all, got %q", out.String())
	}
}

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

func body_projectTsPreflightPart3Test_39(t *testing.T, readiness *projectreadiness.Result, gapCode string, offered bool) {
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", gapCode, readiness, strings.NewReader("cancel\n"), &out)
	if offered {
		if result.NoChoicesOffered {
			t.Fatalf("NoChoicesOffered = true for executable gap %q, want the menu to have opened: %+v", gapCode, result)
		}
		if out.Len() == 0 {
			t.Fatalf("expected the menu to have been printed for executable gap %q", gapCode)
		}
		return
	}
	if !result.NoChoicesOffered {
		t.Fatalf("NoChoicesOffered = false for runtime-boundary gap %q, want true: %+v", gapCode, result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output at all for runtime-boundary gap %q, got %q", gapCode, out.String())
	}
}

// TestRunCompilerSetupOfferRequiresPolicyFirst pins O3's defense-in-depth
// precondition, mirroring RunPrepareCompilerMiseSetup's own
// TestRunPrepareCompilerMiseSetupRequiresPolicyFirst: even when the readiness
// snapshot's compiler check genuinely offers an executable menu entry,
// RunCompilerSetupOffer must refuse before ever printing a prompt while
// readiness.Checks.Policy has not passed.
func TestRunCompilerSetupOfferRequiresPolicyFirst(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Policy:         projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing},
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", projectreadiness.GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)

	if !result.PolicyRequired {
		t.Fatalf("PolicyRequired = false, want true: %+v", result)
	}
	if result.NoChoicesOffered || result.Cancelled || result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output at all while a policy gap coexists with a compiler gap, got %q", out.String())
	}
}

func TestRunCompilerSetupOfferNeverOpensWhenReadinessRuntimeBlocks(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Policy:         projectreadiness.Check{State: projectreadiness.Pass},
			Runtime:        projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeMissing},
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", projectreadiness.GapTypescriptCompilerMissing, readiness, strings.NewReader("cancel\n"), &out)
	if result.RuntimeGapCode != projectreadiness.GapNodeMissing {
		t.Fatalf("RuntimeGapCode = %q, want %q: a compiler gapCode must not open the install menu while readiness.Checks.Runtime independently fails; result=%+v", result.RuntimeGapCode, projectreadiness.GapNodeMissing, result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output while a runtime-boundary gap blocks compiler setup, got %q", out.String())
	}
}
