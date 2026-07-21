package claudehooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGatePrCreation_NonCreateBashNoOp verifies that a Bash command unrelated
// to `gh pr create` is a silent no-op, and does not even invoke `mise`.
func TestGatePrCreation_NonCreateBashNoOp(t *testing.T) {
	stdout := runGatePrCreation(t, "Bash", "git status", true)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for an unrelated Bash command; got: %q", stdout)
	}
}

// TestGatePrCreation_EmptyCommandNoOp verifies a missing/empty tool_input.command is a no-op.
func TestGatePrCreation_EmptyCommandNoOp(t *testing.T) {
	stdout := runGatePrCreation(t, "Bash", "", true)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for an empty command; got: %q", stdout)
	}
}

// TestGatePrCreation_BashCreateAllowsWhenCiPasses verifies that `gh pr create`
// is allowed through (no deny JSON) when `mise run ci` succeeds.
func TestGatePrCreation_BashCreateAllowsWhenCiPasses(t *testing.T) {
	stdout := runGatePrCreation(t, "Bash", "gh pr create --title x --body y", true)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout (allow) when ci passes; got: %q", stdout)
	}
}

// TestGatePrCreation_BashCreateDeniesWhenCiFails verifies that `gh pr create`
// is denied with a hookSpecificOutput payload when `mise run ci` fails.
func TestGatePrCreation_BashCreateDeniesWhenCiFails(t *testing.T) {
	stdout := runGatePrCreation(t, "Bash", "gh pr create --title x --body y", false)
	assertDenyDecision(t, stdout)
}

// TestGatePrCreation_McpCreatePullRequestDeniesWhenCiFails verifies the gate
// also covers the GitHub MCP create_pull_request tool path, used in
// environments where PR creation doesn't go through a Bash `gh` invocation.
func TestGatePrCreation_McpCreatePullRequestDeniesWhenCiFails(t *testing.T) {
	stdout := runGatePrCreation(t, "mcp__github__create_pull_request", "", false)
	assertDenyDecision(t, stdout)
}

// TestGatePrCreation_McpCreatePullRequestAllowsWhenCiPasses mirrors the above
// for the passing-ci case.
func TestGatePrCreation_McpCreatePullRequestAllowsWhenCiPasses(t *testing.T) {
	stdout := runGatePrCreation(t, "mcp__github__create_pull_request", "", true)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout (allow) when ci passes; got: %q", stdout)
	}
}

// TestGatePrCreation_OtherToolNoOp verifies a tool that is neither Bash nor
// the MCP create_pull_request tool is left alone.
func TestGatePrCreation_OtherToolNoOp(t *testing.T) {
	stdout := runGatePrCreation(t, "Read", "", false)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for an unrelated tool; got: %q", stdout)
	}
}

func assertDenyDecision(t *testing.T, stdout string) {
	t.Helper()
	var payload struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected deny JSON on stdout, got unparseable output %q: %v", stdout, err)
	}
	if payload.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("expected permissionDecision=deny, got %q (stdout: %s)", payload.HookSpecificOutput.PermissionDecision, stdout)
	}
	if payload.HookSpecificOutput.PermissionDecisionReason == "" {
		t.Fatalf("expected a non-empty deny reason; got stdout: %s", stdout)
	}
}

// runGatePrCreation execs gate-pr-creation.sh with a synthetic PreToolUse
// payload, stubbing `mise` on PATH so the test never runs the real (slow)
// CI suite — it only needs to prove the hook reacts correctly to mise's exit
// code.
func runGatePrCreation(t *testing.T, toolName, command string, ciPasses bool) string {
	t.Helper()
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "gate-pr-creation.sh"))
	if err != nil {
		t.Fatal(err)
	}

	fakeBin := t.TempDir()
	fakeMise := filepath.Join(fakeBin, "mise")
	exitCode := "0"
	if !ciPasses {
		exitCode = "1"
	}
	if err := os.WriteFile(fakeMise, []byte("#!/bin/sh\nexit "+exitCode+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"tool_name": toolName,
		"tool_input": map[string]string{
			"command": command,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("gate-pr-creation.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return stdout.String()
}
