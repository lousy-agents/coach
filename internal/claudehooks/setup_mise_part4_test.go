package claudehooks

import (
	"os"

	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_NoEmptyPathElement verifies that the hook never writes an empty
// element into PATH. An empty element resolves to the current working
// directory, which would let a file in the repo shadow a real command for every
// later Bash call.
func TestSetupMise_NoEmptyPathElement(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", "")), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written: %v", err)
	}
	exported := strings.TrimSpace(string(data))
	if !strings.Contains(exported, localBin) {
		t.Fatalf("expected CLAUDE_ENV_FILE to prepend user local bin path; got:\n%s", exported)
	}

	value := strings.TrimPrefix(exported, "export PATH=")
	for _, elem := range strings.Split(value, ":") {
		if elem == "" || elem == "''" || elem == `""` {
			t.Fatalf("PATH contains an empty element (resolves to cwd); got:\n%s", exported)
		}
	}
}

// TestSetupMise_UpgradesStaleVersion verifies that the SessionStart hook
// installs the mise version pinned in mise.toml when an older mise binary is
// already on PATH. This is an acceptance test for .claude/hooks/setup-mise.sh.
func TestSetupMise_UpgradesStaleVersion(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	oldMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(oldMise, []byte(fakeMiseScript("2024.1.1", "/old/mise/bin")), 0755); err != nil {
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
		t.Fatalf("npm was not invoked to upgrade the stale mise binary: %v", err)
	}
	args, _ := os.ReadFile(npmCalled)
	if !strings.Contains(string(args), "mise@2026.7.7") {
		t.Fatalf("npm install did not target the expected mise version: %s", args)
	}
}
