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

// TestSetupMise_UnparseableVersionTriggersInstall verifies that when mise is on
// PATH but --version output has no YYYY.M.PATCH token, the hook does not abort
// under set -o pipefail (grep exit 1) and instead falls through to npm install.
func TestSetupMise_UnparseableVersionTriggersInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

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

// TestSetupMise_FindsMiseInHomeLocalBin verifies that a previous install under
// $HOME/.local/bin is detected even when that directory is not already on PATH.
// Cloud sessions may cache the binary on disk without persisting PATH; without
// an early PATH prepend the hook would re-run npm install every SessionStart.
func TestSetupMise_FindsMiseInHomeLocalBin(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(localBin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")

	runHook(t, home, project, envFile, npmDir)

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm was invoked even though a current mise already exists in $HOME/.local/bin")
	}
	_ = bin
}
