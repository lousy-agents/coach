package claudehooks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestSetupMise_StdoutSilentDuringInstall verifies that the hook stays silent on
// stdout on the fresh-install path, where npm is actually invoked. This is the
// path a first cloud session takes, so npm's progress output must not reach
// stdout and get injected into the conversation context.
func TestSetupMise_StdoutSilentDuringInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// No mise on PATH, so the hook must install it via npm.
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
