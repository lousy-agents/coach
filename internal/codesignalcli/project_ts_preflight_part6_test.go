package codesignalcli

import (
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// TestRunCompilerSetupOfferRequiresPolicyFirst pins O3's defense-in-depth
// precondition, mirroring RunPrepareCompilerMiseSetup's own
// TestRunPrepareCompilerMiseSetupRequiresPolicyFirst: even when the readiness
// snapshot's compiler check genuinely offers an executable menu entry,
// RunCompilerSetupOffer must refuse before ever printing a prompt while
// readiness.Checks.Policy has not passed.
func TestRunCompilerSetupOfferRequiresPolicyFirst(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)

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
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader("cancel\n"), &out)
	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Choice != "" {
		t.Fatalf("Choice = %q, want empty on a cancelled selection", result.Choice)
	}
}

func TestRunCompilerSetupOfferNeverOpensWhenReadinessRuntimeBlocks(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Runtime:        ReadinessCheck{State: ReadinessFail, Code: GapNodeMissing},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader("cancel\n"), &out)
	if result.RuntimeGapCode != GapNodeMissing {
		t.Fatalf("RuntimeGapCode = %q, want %q: a compiler gapCode must not open the install menu while readiness.Checks.Runtime independently fails; result=%+v", result.RuntimeGapCode, GapNodeMissing, result)
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
