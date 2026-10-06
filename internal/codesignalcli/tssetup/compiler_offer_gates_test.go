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
			checkRunCompilerSetupOfferReportsNo(t, readiness)
		})
	}
}

func checkRunCompilerSetupOfferReportsNo(t *testing.T, readiness *projectreadiness.Result) {
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
