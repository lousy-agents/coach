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

// TestGatePrCreation_BashCreateAllowsOnCleanTree verifies `gh pr create` passes
// on a clean tree. The gate no longer runs any validation of its own -- the
// required CI checks do that -- so a clean tree is the whole condition.
func TestGatePrCreation_BashCreateAllowsOnCleanTree(t *testing.T) {
	stdout := runGatePrCreation(t, "Bash", "gh pr create --title x --body y", true)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout (allow) on a clean tree; got: %q", stdout)
	}
}

// TestGatePrCreation_McpCreatePullRequestAllowsOnCleanTree mirrors the above for
// the MCP path, which is the only one available in environments without gh.
func TestGatePrCreation_McpCreatePullRequestAllowsOnCleanTree(t *testing.T) {
	stdout := runGatePrCreation(t, "mcp__github__create_pull_request", "", true)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout (allow) on a clean tree; got: %q", stdout)
	}
}

// TestGatePrCreation_MentionIsNotAnInvocation guards a false positive that was
// observed in practice: the hook receives one flat command string, so an
// unanchored search cannot tell `gh pr create` being run from a commit message
// that merely discusses it.
func TestGatePrCreation_MentionIsNotAnInvocation(t *testing.T) {
	// Deliberately a dirty tree. With a clean one the gate allows whether or not
	// the filter matched, so the assertion could not tell an anchored filter from
	// an unanchored one -- and a mutation removing the anchor survived it.
	stdout := runGatePrCreation(t, "Bash", `git commit -m "document the gh pr create gate"`, false)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("a mention must not be gated; got: %q", stdout)
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
// payload. cleanTree stubs `git status --porcelain` as empty or dirty; the hook
// runs no task runner at all any more, so there is nothing else to stub.
func runGatePrCreation(t *testing.T, toolName, command string, cleanTree bool) string {
	t.Helper()
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "gate-pr-creation.sh"))
	if err != nil {
		t.Fatal(err)
	}

	fakeBin := t.TempDir()

	// Stub git. Without this the hook reads the real repository, so these tests
	// would pass or fail depending on whether the checkout happens to be dirty.
	porcelain := ""
	if !cleanTree {
		porcelain = "echo ' M pkg/semantics/analyzer.go'\n"
	}
	fakeGit := filepath.Join(fakeBin, "git")
	if err := os.WriteFile(fakeGit,
		[]byte("#!/bin/sh\ncase \"$*\" in\n  'status --porcelain') "+porcelain+";;\nesac\nexit 0\n"), 0755); err != nil {
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
