package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_InstallFallbackToGoNode verifies that when bare `mise install`
// fails, the hook retries with `mise install go node` and still writes PATH.
func TestSetupMise_InstallFallbackToGoNode(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	logPath := filepath.Join(tmp, "mise-log")
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScriptRecording(logPath, "2026.7.7", localBin, false, true)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed on install fallback path: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected mise invocations to be logged: %v", err)
	}
	log := string(logData)
	if !hasLogLine(log, "install") {
		t.Fatalf("expected bare install attempt; log:\n%s", log)
	}
	if !hasLogLine(log, "install go node") {
		t.Fatalf("expected fallback install go node; log:\n%s", log)
	}
	if !strings.Contains(string(stderr), "installing go and node") {
		t.Fatalf("expected stderr note about fallback; got: %q", stderr)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written after fallback: %v", err)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout; got: %q", stdout)
	}
}

func absScript(t *testing.T) string {
	t.Helper()
	scriptPath := filepath.Join("..", "..", ".claude", "hooks", "setup-mise.sh")
	absScript, err := filepath.Abs(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	return absScript
}

func runHook(t *testing.T, home, project, envFile, path string) []byte {
	t.Helper()
	cmd := exec.Command("bash", absScript(t))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CLAUDE_CODE_REMOTE=true",
		"CLAUDE_PROJECT_DIR="+project,
		"CLAUDE_ENV_FILE="+envFile,
	)
	cmd.Env = append(cmd.Env, "PATH="+path+":/usr/bin:/bin")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup-mise.sh failed: %v\n%s", err, out)
	}
	return out
}
