package projectreadiness

import (
	"testing"
)

// TestGapCodeMappings proves StatusForGapCode and NextActionForGapCode agree
// with the frozen gap-code table for the 11 gap codes that name an action of their
// own (the two policy codes are in the test below), not just the ones
// reachable through today's checks. GapTypescriptCompilerMissing is
// reachable via resolveCompiler; GapPackageManagerVersionUnverifiable,
// GapPackageManagerVersionUnsupported, and GapPackageManagerConfigUnverifiable
// are reachable via checkPackageManager's npm/pnpm/Bun/Yarn detection and via
// CheckProjectReadiness's own evaluateMiseSetupChoices call
// (evaluateMiseProjectTrust/evaluateMiseGlobalTrust, see
// tstoolchain's mise_tool_version_test.go and
// tssetup's mise_install_acceptance_test.go trust-gate specs).
// GapTypescriptVersionMismatch, GapTypescriptVersionConflict, and
// GapPackageManagerAmbiguous remain unreachable via the CLI until later work,
// but every entry in the mapping table must not silently drift regardless.
func TestGapCodeMappings(t *testing.T) {
	cases := []struct {
		code           string
		wantStatus     Status
		wantNextAction string
	}{
		{GapUnsupportedRepositoryShape, StatusOutsideSupport, "confirm_repository_shape"},
		{GapNodeMissing, StatusNeedsPrerequisite, "install_supported_runtime"},
		{GapNodeUnsupported, StatusNeedsPrerequisite, "install_supported_runtime"},
		{GapNodeUnverifiable, StatusNeedsPrerequisite, "repair_runtime_probe"},
		{GapTypescriptCompilerMissing, StatusNeedsPrerequisite, "prepare_compiler"},
		{GapTypescriptVersionMismatch, StatusNeedsPrerequisite, "prepare_compiler"},
		{GapTypescriptVersionConflict, StatusNeedsPrerequisite, "prepare_compiler"},
		{GapPackageManagerAmbiguous, StatusNeedsPrerequisite, "resolve_package_manager"},
		{GapPackageManagerVersionUnverifiable, StatusNeedsPrerequisite, "resolve_package_manager"},
		{GapPackageManagerVersionUnsupported, StatusNeedsPrerequisite, "resolve_package_manager"},
		{GapPackageManagerConfigUnverifiable, StatusNeedsPrerequisite, "resolve_package_manager"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			checkGapCodeMappings(t, tc)
		})
	}
}

func checkGapCodeMappings(t *testing.T, tc struct {
	code           string
	wantStatus     Status
	wantNextAction string
}) {
	if got := StatusForGapCode(tc.code); got != tc.wantStatus {
		t.Fatalf("statusForGapCode(%q) = %q, want %q", tc.code, got, tc.wantStatus)
	}
	kind, ok := NextActionForGapCode(tc.code)
	if !ok {
		t.Fatalf("nextActionForGapCode(%q) returned ok=false, want %q", tc.code, tc.wantNextAction)
	}
	if kind != tc.wantNextAction {
		t.Fatalf("nextActionForGapCode(%q) = %q, want %q", tc.code, kind, tc.wantNextAction)
	}
}

// A policy gap names its action through its remedy, never through its code: the
// same code means "write a new policy", "restore the committed one" or "neither"
// depending on what is on disk and in the index. So the code alone names no action,
// and a caller that asks it gets none instead of a stale author_policy.
func TestAPolicyGapNamesItsActionOnlyThroughItsRemedy(t *testing.T) {
	for _, code := range []string{GapPolicyMissing, GapPolicyInvalid} {
		t.Run(code, func(t *testing.T) {
			if got := StatusForGapCode(code); got != StatusNeedsPolicy {
				t.Errorf("StatusForGapCode(%q) = %q, want %q", code, got, StatusNeedsPolicy)
			}
			if kind, ok := NextActionForGapCode(code); ok {
				t.Errorf("NextActionForGapCode(%q) = %q, true; want no action from the code alone", code, kind)
			}
			wants := map[PolicyRemedy]string{RemedyAuthor: "author_policy", RemedyRestore: "restore_policy"}
			for remedy, want := range wants {
				if kind, ok := NextActionForGap(code, remedy); !ok || kind != want {
					t.Errorf("NextActionForGap(%q, remedy %d) = %q, %v; want %q", code, remedy, kind, ok, want)
				}
			}
			if kind, ok := NextActionForGap(code, RemedyNone); ok {
				t.Errorf("NextActionForGap(%q, RemedyNone) = %q, true; want none", code, kind)
			}
		})
	}
}
