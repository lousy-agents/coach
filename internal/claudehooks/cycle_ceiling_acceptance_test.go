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

// The per-task cycle cap lives in the command as prose, so an orchestrator that
// loses track of it -- or never had it, on a harness that reads the file
// differently -- loops until the budget is gone. This hook is the mechanical
// floor under that rule: a total-invocation ceiling the model cannot talk its
// way past. It is deliberately cruder than the prose rule, which knows about
// tasks; this only knows how many reviews a run has spent.
func runCeiling(sessionID string, invocations int, stateDir string) (blocked bool, reason string) {
	GinkgoHelper()

	path, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "enforce-cycle-ceiling.sh"))
	Expect(err).NotTo(HaveOccurred())

	for i := 0; i < invocations; i++ {
		body, err := json.Marshal(map[string]any{"session_id": sessionID})
		Expect(err).NotTo(HaveOccurred())

		cmd := exec.Command("bash", path)
		cmd.Stdin = bytes.NewReader(body)
		cmd.Env = append(os.Environ(), "COACH_CYCLE_STATE_DIR="+stateDir)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		Expect(cmd.Run()).To(Succeed(), "hook exited non-zero; stderr: %s", stderr.String())

		if out := strings.TrimSpace(stdout.String()); out != "" {
			var payload struct {
				Decision string `json:"decision"`
				Reason   string `json:"reason"`
			}
			Expect(json.Unmarshal([]byte(out), &payload)).To(Succeed())
			Expect(payload.Decision).To(Equal("block"))
			blocked, reason = true, payload.Reason
		}
	}
	return blocked, reason
}

var _ = Describe("reviewer cycle ceiling", func() {
	var stateDir string

	BeforeEach(func() { stateDir = GinkgoT().TempDir() })

	When("a run stays within a plausible number of reviews", func() {
		It("does not interfere", func() {
			blocked, _ := runCeiling("session-a", 8, stateDir)
			Expect(blocked).To(BeFalse(),
				"a real multi-task run makes many legitimate reviews; the ceiling must not fire on those")
		})
	})

	When("a run spends far more reviews than any plan justifies", func() {
		It("blocks, whatever the orchestrator believes about its own cap", func() {
			blocked, reason := runCeiling("session-b", 40, stateDir)
			Expect(blocked).To(BeTrue())
			Expect(reason).To(ContainSubstring("ceiling"))
			Expect(reason).To(MatchRegexp(`(?i)stop|halt|abandon`),
				"a block that does not say what to do next just gets retried")
		})
	})

	// Two runs in one working copy must not inherit each other's count, or the
	// second run starts pre-exhausted and the ceiling fires on a healthy loop.
	When("a different run uses the same checkout", func() {
		It("counts independently", func() {
			blockedA, _ := runCeiling("session-c", 40, stateDir)
			Expect(blockedA).To(BeTrue())
			blockedB, _ := runCeiling("session-d", 3, stateDir)
			Expect(blockedB).To(BeFalse(), "one exhausted run must not poison another")
		})
	})

	// Losing the counter must not silently disarm the ceiling. The path is a
	// directory *under an existing file*, which mkdir cannot create for any
	// user -- a merely absent path is creatable when running as root, so it
	// would not exercise this branch at all.
	When("the counter cannot be written", func() {
		It("blocks rather than allowing an uncounted loop", func() {
			blockedFile := filepath.Join(GinkgoT().TempDir(), "not-a-dir")
			Expect(os.WriteFile(blockedFile, []byte("x"), 0o600)).To(Succeed())

			blocked, reason := runCeiling("session-e", 1, filepath.Join(blockedFile, "state"))
			Expect(blocked).To(BeTrue(),
				"an unwritable counter means the ceiling is not enforcing; fail closed")
			Expect(reason).To(MatchRegexp(`(?i)cannot be enforced`))
		})
	})
})
