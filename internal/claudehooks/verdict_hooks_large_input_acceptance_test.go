package claudehooks

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// oversizedBody is far larger than any pipe buffer, so a hook that streams it
// through a pipeline whose reader exits early is killed by SIGPIPE
// deterministically rather than only under unlucky scheduling.
var oversizedBody = strings.Repeat("padding line that is irrelevant to the verdict\n", 80_000)

func runVerdictHook(message string) hookRun {
	GinkgoHelper()
	return runHookRaw("verify-review-verdict.sh", map[string]any{"last_assistant_message": message})
}

func runRelayHook(prompt string) hookRun {
	GinkgoHelper()
	return runHookRaw("verify-context-relay.sh", map[string]any{
		"tool_input": map[string]string{"subagent_type": "task-implementer", "prompt": prompt},
	})
}

func expectAllowed(run hookRun) {
	GinkgoHelper()
	Expect(run.exitCode).To(Equal(0), "hook must exit 0 (%s); stderr: %q", run.status, run.stderr)
	Expect(strings.TrimSpace(run.stdout)).To(BeEmpty())
}

func expectVerdictBlocked(run hookRun) {
	GinkgoHelper()
	Expect(run.exitCode).To(Equal(0), "hook must exit 0 (%s); stderr: %q", run.status, run.stderr)
	var payload struct {
		Decision string `json:"decision"`
	}
	Expect(json.Unmarshal([]byte(run.stdout), &payload)).To(Succeed(), "stdout: %q", run.stdout)
	Expect(payload.Decision).To(Equal("block"))
}

func expectRelayDenied(run hookRun) {
	GinkgoHelper()
	Expect(run.exitCode).To(Equal(0), "hook must exit 0 (%s); stderr: %q", run.status, run.stderr)
	var payload struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	Expect(json.Unmarshal([]byte(run.stdout), &payload)).To(Succeed(), "stdout: %q", run.stdout)
	Expect(payload.HookSpecificOutput.PermissionDecision).To(Equal("deny"))
}

var _ = Describe("reviewer verdict contract with an oversized reply", func() {
	When("a well-formed verdict is followed by a multi-megabyte body", func() {
		DescribeTable("it is allowed through rather than crashing the hook",
			func(message string) { expectAllowed(runVerdictHook(message)) },
			Entry("PASS", "PASS\n"+oversizedBody),
			Entry("FINDINGS", "FINDINGS\n## Reviewer Findings\n"+oversizedBody),
			Entry("PASS after leading blank lines", "\n\n\nPASS -- ok\n"+oversizedBody),
		)
	})

	When("a malformed reply is followed by a multi-megabyte body", func() {
		DescribeTable("it is still blocked with the JSON decision",
			func(message string) { expectVerdictBlocked(runVerdictHook(message)) },
			Entry("prose first, verdict later", "I reviewed the diff.\n"+oversizedBody+"PASS\n"),
			Entry("PASSWORD", "PASSWORD\n"+oversizedBody),
		)
	})
})

var _ = Describe("findings relay contract with an oversized prompt", func() {
	const rework = "Re-review the reviewer findings and fix them.\n"

	When("a rework prompt matches the keywords early and is multi-megabyte", func() {
		It("is denied when the findings block is missing", func() {
			expectRelayDenied(runRelayHook(rework + oversizedBody))
		})

		// The two allow specs below are guards, not red-then-green: on the
		// unfixed hook the outer pipeline dies first and the hook exits 0 for
		// the wrong reason. Only an inner-pipeline regression fails them.
		It("guard: is allowed when the findings block is at the very end", func() {
			expectAllowed(runRelayHook(rework + oversizedBody + "## Reviewer Findings\n1. fix x\n"))
		})

		It("guard: is allowed when the findings block is at the start", func() {
			expectAllowed(runRelayHook("## Reviewer Findings\n1. fix x\n" + rework + oversizedBody))
		})
	})

	When("a multi-megabyte prompt is not rework", func() {
		It("guard: stays out of the way", func() {
			expectAllowed(runRelayHook("Implement task T1.\n" + oversizedBody))
		})
	})
})
