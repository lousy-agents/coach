package claudehooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_TrustFailureContinuesInstall verifies that a non-zero mise trust
// exit does not abort bootstrap: install still runs and CLAUDE_ENV_FILE is written.
func TestSetupMise_TrustFailureContinuesInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	logPath := filepath.Join(tmp, "mise-log")
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScriptRecording(logPath, "2026.7.7", localBin, true, false)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed after trust failure: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected mise invocations to be logged: %v", err)
	}
	log := string(logData)
	if !strings.Contains(log, "trust ") && !strings.Contains(log, "trust\n") {
		t.Fatalf("expected trust to be attempted; log:\n%s", log)
	}
	if !hasLogLine(log, "install") {
		t.Fatalf("expected bare install after trust failure; log:\n%s", log)
	}
	if !strings.Contains(string(stderr), "mise trust failed") {
		t.Fatalf("expected stderr warning about trust failure; got: %q", stderr)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written: %v", err)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout; got: %q", stdout)
	}
}
