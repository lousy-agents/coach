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

	// A current mise that reports no tool bin paths at all.
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
