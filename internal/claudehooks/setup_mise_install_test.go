package claudehooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_InstallsWhenMissing verifies that the hook installs mise via npm
// when there is no mise binary on PATH.
func TestSetupMise_InstallsWhenMissing(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte(fakeNpmScript(npmCalled, newMise, localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("npm was not invoked to install mise: %v", err)
	}
	args, _ := os.ReadFile(npmCalled)
	if !strings.Contains(string(args), "mise@2026.7.7") {
		t.Fatalf("npm install did not target the expected mise version: %s", args)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written after fresh install: %v", err)
	}
}

// TestSetupMise_UpgradesStaleVersion verifies that the SessionStart hook
// installs the mise version pinned in mise.toml when an older mise binary is
// already on PATH. This is an acceptance test for .claude/hooks/setup-mise.sh.
func TestSetupMise_UpgradesStaleVersion(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// Stale mise already on PATH.
	oldMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(oldMise, []byte(fakeMiseScript("2024.1.1", "/old/mise/bin")), 0755); err != nil {
		t.Fatal(err)
	}

	// Fake npm records its invocation and writes a newer mise binary.
	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte(fakeNpmScript(npmCalled, newMise, localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("npm was not invoked to upgrade the stale mise binary: %v", err)
	}
	args, _ := os.ReadFile(npmCalled)
	if !strings.Contains(string(args), "mise@2026.7.7") {
		t.Fatalf("npm install did not target the expected mise version: %s", args)
	}
}

// TestSetupMise_UnparseableVersionTriggersInstall verifies that when mise is on
// PATH but --version output has no YYYY.M.PATCH token, the hook does not abort
// under set -o pipefail (grep exit 1) and instead falls through to npm install.
func TestSetupMise_UnparseableVersionTriggersInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// mise exists but reports a version string the hook cannot parse.
	weirdMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(weirdMise, []byte(fakeMiseScript("not-a-semver-build", "/old/bin")), 0755); err != nil {
		t.Fatal(err)
	}

	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte(fakeNpmScript(npmCalled, newMise, localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("npm was not invoked after unparseable mise --version: %v", err)
	}
}
