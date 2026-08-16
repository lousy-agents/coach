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

// traceRows runs a hook with COACH_HOOK_TRACE pointed at a temp file and
// returns the rows it appended, plus anything the hook wrote to stderr.
func traceRows(script string, payload map[string]any, tracePath string) ([][]string, string) {
	GinkgoHelper()

	path, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", script))
	Expect(err).NotTo(HaveOccurred())
	body, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("bash", path)
	cmd.Stdin = bytes.NewReader(body)
	cmd.Env = append(os.Environ(), "COACH_HOOK_TRACE="+tracePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	Expect(cmd.Run()).To(Succeed(), "hook exited non-zero; stderr: %s", stderr.String())

	raw, err := os.ReadFile(tracePath)
	if err != nil {
		return nil, stderr.String()
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line != "" {
			rows = append(rows, strings.Split(line, "\t"))
		}
	}
	return rows, stderr.String()
}

var _ = Describe("hook trace", func() {
	var tracePath string

	BeforeEach(func() { tracePath = filepath.Join(GinkgoT().TempDir(), "trace.tsv") })

	// The whole point is telling "never fired" apart from "fired and allowed".
	// A hook that only traces past its early-exit filter leaves those two cases
	// byte-identical, which is the ambiguity being bought out.
	When("a hook runs but the payload does not match it", func() {
		It("still records that it fired", func() {
			rows, _ := traceRows("verify-context-relay.sh", map[string]any{
				"tool_input": map[string]string{"subagent_type": "task-reviewer", "prompt": "x"},
			}, tracePath)
			Expect(rows).To(HaveLen(1))
			Expect(rows[0][0]).To(Equal("relay"))
			Expect(rows[0][2]).To(Equal("fired"))
		})
	})

	When("a hook reaches a decision", func() {
		It("records the decision alongside the firing", func() {
			rows, _ := traceRows("verify-review-verdict.sh",
				map[string]any{"last_assistant_message": "garbage"}, tracePath)
			var decisions []string
			for _, r := range rows {
				decisions = append(decisions, r[2])
			}
			Expect(decisions).To(ContainElement("fired"))
			Expect(decisions).To(ContainElement("block"))
		})
	})

	// Blocking a reviewer that was already re-run because of a block is an
	// unbounded machine-driven retry inside one orchestrator cycle -- the cycle
	// cap never sees it, because no cycle completes.
	When("the runtime is already re-running an agent a hook blocked", func() {
		It("defers instead of blocking again", func() {
			rows, _ := traceRows("verify-review-verdict.sh", map[string]any{
				"last_assistant_message": "garbage", "stop_hook_active": true,
			}, tracePath)
			var decisions []string
			for _, r := range rows {
				decisions = append(decisions, r[2])
			}
			Expect(decisions).To(ContainElement("defer-already-blocked"))
			Expect(decisions).NotTo(ContainElement("block"))
		})
	})

	// bash reports a failed `>>` on the shell's own stderr before a trailing
	// 2>/dev/null applies, so an unwritable path used to make every invocation
	// spray diagnostics into the hook's output channel.
	// The unwritable path is a file used as a directory, not a merely absent
	// one: an absent path is creatable, and a hardcoded absolute path is shared
	// state between tests -- the cycle-ceiling hook's mkdir once created
	// /nonexistent-dir and silently made this assertion pass for the wrong
	// reason.
	When("the trace path cannot be written", func() {
		It("stays silent and still decides", func() {
			blocked := filepath.Join(GinkgoT().TempDir(), "not-a-dir")
			Expect(os.WriteFile(blocked, []byte("x"), 0o600)).To(Succeed())

			rows, stderr := traceRows("verify-review-verdict.sh",
				map[string]any{"last_assistant_message": "PASS"},
				filepath.Join(blocked, "trace.tsv"))
			Expect(rows).To(BeEmpty())
			Expect(stderr).To(BeEmpty(), "a broken trace path must not pollute hook stderr")
		})
	})

	When("the trace is not requested", func() {
		It("writes nothing at all", func() {
			path, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "verify-review-verdict.sh"))
			Expect(err).NotTo(HaveOccurred())
			body, err := json.Marshal(map[string]any{"last_assistant_message": "PASS"})
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("bash", path)
			cmd.Stdin = bytes.NewReader(body)
			cmd.Env = append(os.Environ(), "COACH_HOOK_TRACE=")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			Expect(cmd.Run()).To(Succeed())
			Expect(stderr.String()).To(BeEmpty())
			Expect(tracePath).NotTo(BeAnExistingFile())
		})
	})
})
