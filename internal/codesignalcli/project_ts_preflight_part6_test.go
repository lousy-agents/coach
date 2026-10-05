package codesignalcli

import (
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

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
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
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

// TestRunCompilerSetupOfferCancelsOnExplicitCancelChoice pins that typing
// the literal "cancel" choice, not merely an unreadable/blank answer,
// selects the same Cancelled outcome with no Choice recorded. The fixture
// must offer a genuine, non-cancel choice or the prompt this pins would
// never open at all.
func TestRunCompilerSetupOfferCancelsOnExplicitCancelChoice(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Policy:         projectreadiness.Check{State: projectreadiness.Pass},
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", projectreadiness.GapTypescriptCompilerMissing, readiness, strings.NewReader("cancel\n"), &out)
	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Choice != "" {
		t.Fatalf("Choice = %q, want empty on a cancelled selection", result.Choice)
	}
}

func TestRunCompilerSetupOfferNeverOpensWhenReadinessRuntimeBlocks(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Policy:         projectreadiness.Check{State: projectreadiness.Pass},
			Runtime:        projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapNodeMissing},
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
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

func TestWrapCompilerUnresolvedErrorWithReadinessLeavesOtherErrorsUnchanged(t *testing.T) {
	plain := &gitrepo.OperationalError{Message: "boom"}
	if got := WrapCompilerUnresolvedErrorWithReadiness(plain, ".", "HEAD", "project.json"); got != error(plain) {
		t.Fatalf("WrapCompilerUnresolvedErrorWithReadiness(non-CompilerUnresolvedError) = %v, want the original error unchanged", got)
	}
}
