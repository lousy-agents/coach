package claudehooks

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyContextRelay_NonImplementerAllows verifies the hook only applies
// to task-implementer delegations; any other subagent_type is a no-op
// regardless of prompt content.
func TestVerifyContextRelay_NonImplementerAllows(t *testing.T) {
	stdout := runVerifyContextRelay(t, "task-reviewer", "Re-delegating after reviewer findings, no heading included.")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for a non-task-implementer subagent_type; got: %q", stdout)
	}
}

// TestVerifyContextRelay_FirstTimeDelegationWithDomainWordAllows verifies
// that a first-time delegation whose prompt happens to use this repo's own
// domain vocabulary (pkg/semantics' Result.Findings) is not mistaken for a
// rework re-delegation, since it names neither the reviewer nor
// redelegation/re-review.
func TestVerifyContextRelay_FirstTimeDelegationWithDomainWordAllows(t *testing.T) {
	stdout := runVerifyContextRelay(t, "task-implementer", "Implement task 3: extend Result.Findings for the new metric.")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for a first-time delegation; got: %q", stdout)
	}
}

// TestVerifyContextRelay_ReworkWithoutHeadingDenies verifies that a
// re-delegation prompt mentioning rework near "reviewer"/"finding" (or
// redelegation/re-review wording) is denied when it omits the literal
// "## Reviewer Findings" heading.
func TestVerifyContextRelay_ReworkWithoutHeadingDenies(t *testing.T) {
	stdout := runVerifyContextRelay(t, "task-implementer", "Re-delegating after the reviewer's findings, please fix the issues.")
	assertRelayDenyDecision(t, stdout)
}

// TestVerifyContextRelay_ReworkWithHeadingAllows verifies that including the
// literal heading satisfies the backstop.
func TestVerifyContextRelay_ReworkWithHeadingAllows(t *testing.T) {
	stdout := runVerifyContextRelay(t, "task-implementer", "Re-delegating after FINDINGS.\n\n## Reviewer Findings\n1. foo.go:12 fix bar")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout once the Reviewer Findings heading is present; got: %q", stdout)
	}
}

// TestVerifyContextRelay_ReDelegateWordingWithoutHeadingDenies verifies the
// re-delegat|re-review branch of the regex independent of "reviewer...finding".
func TestVerifyContextRelay_ReDelegateWordingWithoutHeadingDenies(t *testing.T) {
	stdout := runVerifyContextRelay(t, "task-implementer", "re-review needed, redelegating this task.")
	assertRelayDenyDecision(t, stdout)
}

func assertRelayDenyDecision(t *testing.T, stdout string) {
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

func runVerifyContextRelay(t *testing.T, subagentType, prompt string) string {
	t.Helper()
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "verify-context-relay.sh"))
	if err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"tool_input": map[string]string{
			"subagent_type": subagentType,
			"prompt":        prompt,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verify-context-relay.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return stdout.String()
}
