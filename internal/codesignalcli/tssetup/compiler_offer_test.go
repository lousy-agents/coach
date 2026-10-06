package tssetup

import (
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// TestRunCompilerSetupOfferCancelsOnUnreadableSelection pins the fail-closed
// default: an input stream that produces no answer at all (EOF) must cancel
// rather than falling back to any offered choice. The fixture must offer a
// genuine, non-cancel choice (an installable manifest declaration plus a
// passing package-manager check) or the prompt this pins would never open at
// all.
func TestRunCompilerSetupOfferCancelsOnUnreadableSelection(t *testing.T) {
	readiness := &projectreadiness.Result{
		Checks: projectreadiness.Checks{
			Policy:         projectreadiness.Check{State: projectreadiness.Pass},
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
			PackageManager: projectreadiness.Check{State: projectreadiness.Pass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", projectreadiness.GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)
	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if !strings.Contains(out.String(), "cancel") {
		t.Fatalf("expected the menu to have offered a cancel choice, got %q", out.String())
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
			Compiler:       projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapTypescriptCompilerMissing, DeclaredVersion: tstoolchain.SupportedTypescriptVersions[0]},
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
