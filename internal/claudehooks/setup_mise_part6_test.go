package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"testing"
)

// TestSetupMise_StdoutSilentOnSuccess verifies that the hook produces no stdout
// on success. SessionStart stdout is injected into the conversation context, so
// install progress output must be redirected to stderr or discarded.
func TestSetupMise_StdoutSilentOnSuccess(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScriptWithNoise("2026.7.7", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\necho 'fake npm stdout'\necho 'fake npm stderr' >&2\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout on successful hook run; got: %q", stdout)
	}
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

func TestSetupMise_SkipsInstallWhenCurrent(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("mise 2026.7.7 linux-x64", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

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
