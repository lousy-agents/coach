package tssetup

import (
	"context"
	"strings"
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
// projectcheck.Run reads (projectcheck.Run resolves Node and the
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
			checkRunCompilerSetupOfferNeverOpens(t, readiness, gapCode, offered)
		})
	}
}

func checkRunCompilerSetupOfferNeverOpens(t *testing.T, readiness *projectreadiness.Result, gapCode string, offered bool) {
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
