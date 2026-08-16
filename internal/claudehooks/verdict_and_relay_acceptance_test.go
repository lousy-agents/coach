package claudehooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runHook feeds a synthetic hook payload to one of the committed hook scripts
// and returns its stdout. These hooks are the only mechanical enforcement of
// the reviewer verdict contract and the findings-relay contract, so they are
// exercised as black boxes against real payload shapes.
func runHookPayload(script string, payload map[string]any) string {
	GinkgoHelper()

	path, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", script))
	Expect(err).NotTo(HaveOccurred())

	body, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("bash", path)
	cmd.Stdin = bytes.NewReader(body)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	Expect(cmd.Run()).To(Succeed(), "hook exited non-zero; stderr: %s", stderr.String())
	return stdout.String()
}

func verdictBlocked(message string) bool {
	GinkgoHelper()
	out := runHookPayload("verify-review-verdict.sh", map[string]any{"last_assistant_message": message})
	if strings.TrimSpace(out) == "" {
		return false
	}
	var payload struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	Expect(json.Unmarshal([]byte(out), &payload)).To(Succeed())
	Expect(payload.Decision).To(Equal("block"))
	Expect(payload.Reason).NotTo(BeEmpty(), "a block with no reason gives the subagent nothing to correct")
	return true
}

var _ = Describe("reviewer verdict contract", func() {
	// A reviewer's verdict is the orchestrator's only signal that a task is
	// done. A malformed one that slips through is read as a pass.
	When("a reviewer returns a well-formed verdict", func() {
		DescribeTable("it is allowed through",
			func(message string) { Expect(verdictBlocked(message)).To(BeFalse()) },
			Entry("bare PASS", "PASS"),
			Entry("PASS with a note", "PASS -- verified the integrated diff."),
			Entry("PASS with terminal punctuation", "PASS."),
			Entry("FINDINGS with its block", "FINDINGS\n## Reviewer Findings\n1. fix the guard"),
			Entry("leading blank lines", "\n\nPASS"),
		)
	})

	When("a reviewer returns something that only looks like a verdict", func() {
		DescribeTable("it is blocked so the reviewer re-emits",
			func(message string) { Expect(verdictBlocked(message)).To(BeTrue()) },
			// The trap the hook exists for: a verdict buried after prose reads
			// as a pass to any check anchored per-line rather than to the first
			// non-empty line.
			Entry("prose first, verdict later", "I reviewed the diff.\nIt looks good.\nPASS"),
			// Models routinely emphasise or decorate a verdict.
			Entry("bold", "**PASS**"),
			Entry("markdown heading", "## PASS"),
			Entry("indented", "  PASS"),
			Entry("lowercase", "pass"),
			Entry("lowercase findings", "findings"),
			// PASSED is a different word; treating it as PASS would accept a
			// verdict the contract does not define.
			Entry("PASSED", "PASSED"),
			Entry("empty reply", ""),
		)
	})
})

var _ = Describe("findings relay contract", func() {
	relayDenied := func(subagentType, prompt string) bool {
		GinkgoHelper()
		out := runHookPayload("verify-context-relay.sh", map[string]any{
			"tool_input": map[string]string{"subagent_type": subagentType, "prompt": prompt},
		})
		if strings.TrimSpace(out) == "" {
			return false
		}
		var payload struct {
			HookSpecificOutput struct {
				PermissionDecision string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		Expect(json.Unmarshal([]byte(out), &payload)).To(Succeed())
		return payload.HookSpecificOutput.PermissionDecision == "deny"
	}

	When("rework is delegated after a reviewer returned findings", func() {
		It("is allowed when the findings block is forwarded verbatim", func() {
			Expect(relayDenied("task-implementer",
				"Address the reviewer findings.\n## Reviewer Findings\n1. fix x")).To(BeFalse())
		})

		It("is denied when the findings are paraphrased away", func() {
			Expect(relayDenied("task-implementer",
				"The reviewer had findings; please fix them.")).To(BeTrue())
		})

		// The integration reviewer emits one numbered list under a single
		// heading. Splitting it into one delegation per finding drops the
		// heading, so every such call is refused -- the orchestrator must
		// forward the whole block, which is what the command now says.
		It("is denied when integration findings are split one per delegation", func() {
			Expect(relayDenied("task-implementer",
				"Re-review needed: the integration reviewer flagged that T2 invalidated T1's evidence. Fix it.")).To(BeTrue())
		})
	})

	When("the delegation is not rework", func() {
		It("stays out of the way", func() {
			Expect(relayDenied("task-implementer",
				"Implement task T1 per its acceptance criteria.")).To(BeFalse())
		})
	})
})
