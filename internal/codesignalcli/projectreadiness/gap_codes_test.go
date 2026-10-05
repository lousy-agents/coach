package projectreadiness

import (
	"testing"
)

// TestGapCodeMappings proves StatusForGapCode and NextActionForGapCode agree
// with the frozen gap-code table for all 13 gap codes, not just the ones
// reachable through today's checks. GapTypescriptCompilerMissing is
// reachable via resolveCompiler; GapPackageManagerVersionUnverifiable,
// GapPackageManagerVersionUnsupported, and GapPackageManagerConfigUnverifiable
// are reachable via checkPackageManager's npm/pnpm/Bun/Yarn detection and via
// CheckProjectReadiness's own evaluateMiseSetupChoices call
// (evaluateMiseProjectTrust/evaluateMiseGlobalTrust, see
// project_ts_compiler_mise_version_test.go and
// project_ts_compiler_mise_command_acceptance_test.go's trust-gate specs).
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
		{GapPolicyMissing, StatusNeedsPolicy, "author_policy"},
		{GapPolicyInvalid, StatusNeedsPolicy, "author_policy"},
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
