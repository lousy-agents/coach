package claudehooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSetupMise_SkipsInstallWhenCurrent(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// Current mise already on PATH, with the same version format reported by the real binary.
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("mise 2026.7.7 linux-x64", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	// Fake npm should not be invoked.
	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm was invoked even though the current mise version satisfies min_version")
	}
}

// TestSetupMise_FindsMiseInHomeLocalBin verifies that a previous install under
// $HOME/.local/bin is detected even when that directory is not already on PATH.
// Cloud sessions may cache the binary on disk without persisting PATH; without
// an early PATH prepend the hook would re-run npm install every SessionStart.
func TestSetupMise_FindsMiseInHomeLocalBin(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// Current mise lives only under ~/.local/bin (not on the initial PATH).
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
	// PATH deliberately omits localBin and bin (no other mise).
	runHook(t, home, project, envFile, npmDir)

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm was invoked even though a current mise already exists in $HOME/.local/bin")
	}
	_ = bin
}

// TestSetupMise_LocalNoOp verifies that the hook exits immediately when
// CLAUDE_CODE_REMOTE is not set to true, leaving local sessions unchanged.
func TestSetupMise_LocalNoOp(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// CLAUDE_CODE_REMOTE explicitly cleared: os.Environ() may already carry
	// CLAUDE_CODE_REMOTE=true when this test itself runs inside a Claude Code
	// cloud session, which would otherwise mask the local no-op path.
	envFile := filepath.Join(tmp, "env")
	cmd := exec.Command("bash", absScript(t))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CLAUDE_CODE_REMOTE=",
		"CLAUDE_PROJECT_DIR="+project,
		"CLAUDE_ENV_FILE="+envFile,
	)
	cmd.Env = append(cmd.Env, "PATH="+npmDir+":"+bin+":/usr/bin:/bin")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("setup-mise.sh failed locally: %v\n%s", err, out)
	}

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm should not be invoked when CLAUDE_CODE_REMOTE is unset")
	}
	if _, err := os.Stat(envFile); err == nil {
		t.Fatalf("CLAUDE_ENV_FILE should not be touched when CLAUDE_CODE_REMOTE is unset")
	}
	_ = localBin
}
