package claudehooks

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyReviewVerdict_LeadingPassAllows verifies that a reply beginning
// with a bare PASS is allowed through with no stdout, per
// .claude/hooks/verify-review-verdict.sh's SubagentStop contract for the
// task-reviewer subagent.
func TestVerifyReviewVerdict_LeadingPassAllows(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "PASS — verified the diff and tests.")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for a leading PASS reply; got: %q", stdout)
	}
}

// TestVerifyReviewVerdict_LeadingFindingsAllows verifies that a reply
// beginning with FINDINGS (followed by the required heading and body) is
// allowed through with no stdout.
func TestVerifyReviewVerdict_LeadingFindingsAllows(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "FINDINGS\n\n## Reviewer Findings\n1. foo.go:12 fix bar")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout for a leading FINDINGS reply; got: %q", stdout)
	}
}

// TestVerifyReviewVerdict_MidMessagePassBlocks reproduces the reported bug:
// grep's line-anchored ^ matches PASS on any line of a multi-line reply, not
// only when the reply begins with PASS. A reviewer that prefaces its verdict
// with prose must still be blocked so it re-emits a verdict starting with
// PASS/FINDINGS, per its own system prompt.
func TestVerifyReviewVerdict_MidMessagePassBlocks(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "I reviewed the diff.\nPASS — done")
	assertBlockDecision(t, stdout)
}

// TestVerifyReviewVerdict_MidMessageFindingsBlocks mirrors the PASS case for
// FINDINGS appearing on a later line instead of at the start of the reply.
func TestVerifyReviewVerdict_MidMessageFindingsBlocks(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "Here is my review.\nFINDINGS\n1. foo")
	assertBlockDecision(t, stdout)
}

// TestVerifyReviewVerdict_ProseOnlyBlocks verifies a reply containing neither
// verdict token is blocked.
func TestVerifyReviewVerdict_ProseOnlyBlocks(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "Looks fine to me.")
	assertBlockDecision(t, stdout)
}

// TestVerifyReviewVerdict_EmptyMessageBlocks verifies an empty
// last_assistant_message is blocked rather than silently allowed.
func TestVerifyReviewVerdict_EmptyMessageBlocks(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "")
	assertBlockDecision(t, stdout)
}

// TestVerifyReviewVerdict_WordBoundaryStillEnforced verifies that a leading
// token merely prefixed by PASS/FINDINGS (e.g. PASSWORD) does not satisfy the
// verdict check, guarding against a naive prefix-only anchor fix.
func TestVerifyReviewVerdict_WordBoundaryStillEnforced(t *testing.T) {
	stdout := runVerifyReviewVerdict(t, "PASSWORD reset needed")
	assertBlockDecision(t, stdout)
}

func assertBlockDecision(t *testing.T, stdout string) {
	t.Helper()
	var payload struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected block JSON on stdout, got unparseable output %q: %v", stdout, err)
	}
	if payload.Decision != "block" {
		t.Fatalf("expected decision=block, got %q (stdout: %s)", payload.Decision, stdout)
	}
	if payload.Reason == "" {
		t.Fatalf("expected a non-empty block reason; got stdout: %s", stdout)
	}
}

func runVerifyReviewVerdict(t *testing.T, lastAssistantMessage string) string {
	t.Helper()
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "verify-review-verdict.sh"))
	if err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]string{"last_assistant_message": lastAssistantMessage})
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verify-review-verdict.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return stdout.String()
}
