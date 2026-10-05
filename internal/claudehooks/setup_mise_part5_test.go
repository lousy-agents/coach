package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_UnsetProjectDirDoesNotAbort verifies that an unset
// CLAUDE_PROJECT_DIR does not abort the hook under `set -u`.
//
// It deliberately does not claim to prove "PWD is used to find mise.toml":
// cmd.Dir already starts the shell in the project directory, so `cd "$PWD"` is a
// no-op and deleting the cd entirely would still pass. The load-bearing
// assertion is that the hook completes and writes CLAUDE_ENV_FILE rather than
// dying on an unbound variable.
func TestSetupMise_UnsetProjectDirDoesNotAbort(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	cmd := exec.Command("bash", absScript(t))
	cmd.Dir = project

	base := []string{
		"HOME=" + home,
		"CLAUDE_CODE_REMOTE=true",
		"CLAUDE_ENV_FILE=" + envFile,
		"PATH=" + npmDir + ":" + bin + ":/usr/bin:/bin",
	}
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "HOME="),
			strings.HasPrefix(e, "CLAUDE_CODE_REMOTE="),
			strings.HasPrefix(e, "CLAUDE_PROJECT_DIR="),
			strings.HasPrefix(e, "CLAUDE_ENV_FILE="),
			strings.HasPrefix(e, "PATH="):
			continue
		default:
			base = append(base, e)
		}
	}
	cmd.Env = base

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup-mise.sh failed without CLAUDE_PROJECT_DIR: %v\n%s", err, out)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written when using PWD: %v", err)
	}
}

// TestSetupMise_StdoutSilentDuringInstall verifies that the hook stays silent on
// stdout on the fresh-install path, where npm is actually invoked. This is the
// path a first cloud session takes, so npm's progress output must not reach
// stdout and get injected into the conversation context.
func TestSetupMise_StdoutSilentDuringInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	noisyNpm := fakeNpmScript(npmCalled, newMise, localBin) +
		"echo 'added 1 package in 3s'\necho 'npm notice: something' >&2\n"
	if err := os.WriteFile(npmBin, []byte(noisyNpm), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("expected npm to be invoked on the fresh-install path: %v", err)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout while installing mise; got: %q", stdout)
	}
}
