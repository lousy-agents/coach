package claudehooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Step 0 of the implement-issue command tells the orchestrator to prove the
// hooks are registered by making a call verify-context-relay.sh must deny, and
// to stop the run if that call is allowed. The example prompt it gives and the
// regex the hook triggers on are two halves of one contract held in two files,
// and nothing but this spec connects them.
//
// Drift is silent and inverts the check: an example prompt the regex no longer
// matches produces an allow, step 0 reads the allow as a dead gate, and every
// run stops with environment-failure while the gates are in fact fine. The
// specs in internal/agentworkflows pin the heading and the hook's name; they
// cannot execute the hook, so they cannot catch this.
var _ = Describe("the step 0 liveness probe", func() {
	// exampleProbePrompt returns the prompt the command instructs the
	// orchestrator to send -- the backticked example immediately after the
	// sentence about omitting the heading.
	exampleProbePrompt := func() string {
		GinkgoHelper()
		body, err := os.ReadFile(filepath.Join("..", "..", ".claude", "commands", "implement-issue.md"))
		Expect(err).NotTo(HaveOccurred())
		m := regexp.MustCompile("heading — `([^`]+)`").FindStringSubmatch(string(body))
		Expect(m).NotTo(BeNil(),
			"step 0 no longer offers a concrete example prompt; an orchestrator improvising one may not trip the hook at all")
		return m[1]
	}

	relayDecision := func(prompt string) string {
		GinkgoHelper()
		payload, err := json.Marshal(map[string]any{
			"tool_input": map[string]any{
				"subagent_type": "task-implementer",
				"prompt":        prompt,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		script, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "verify-context-relay.sh"))
		Expect(err).NotTo(HaveOccurred())
		cmd := exec.Command("bash", script)
		cmd.Stdin = bytes.NewReader(payload)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		Expect(cmd.Run()).To(Succeed(), "stderr: %s", stderr.String())
		return stdout.String()
	}

	When("the orchestrator sends the example prompt step 0 specifies", func() {
		It("is denied by the hook it names", func() {
			Expect(relayDecision(exampleProbePrompt())).To(ContainSubstring(`"permissionDecision": "deny"`),
				"step 0 treats an allow as proof the gates are dead, so a drifted example stops every run instead of none")
		})
	})

	// The converse matters as much. If the hook denied regardless of content,
	// the probe would report a live gate in every environment including the one
	// where nothing is registered -- passing for a reason unrelated to what it
	// claims to measure.
	When("the same delegation does carry the heading verbatim", func() {
		It("is allowed, so the denial above is attributable to the omission", func() {
			Expect(relayDecision("Rework this.\n\n## Reviewer Findings\n\n1. foo.go:1 — bar")).
				NotTo(ContainSubstring(`"permissionDecision": "deny"`),
					"a hook that denies unconditionally makes the probe a constant, not a measurement")
		})
	})
})
