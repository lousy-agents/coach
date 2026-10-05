package codesignalcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func coachRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

// TestGapCodeMappings proves statusForGapCode and nextActionForGapCode agree
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
		wantStatus     ReadinessStatus
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
			body_projectReadinessPart5Test_66(t, tc)
		})
	}
}

func packageLockRootEnginesNode(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Packages map[string]struct {
			Engines struct {
				Node string `json:"node"`
			} `json:"engines"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readFileT(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	root, ok := doc.Packages[""]
	if !ok {
		t.Fatalf("%s missing packages[\"\"]", path)
	}
	if root.Engines.Node == "" {
		t.Fatalf("%s packages[\"\"].engines.node is empty", path)
	}
	return root.Engines.Node
}
