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

// ceilingRun is one invocation, keeping stderr as well as the block decision.
// runCeiling asserts that any stdout is a block, which a non-blocking warning
// would trip over.
type ceilingRun struct {
	blocked bool
	reason  string
	stderr  string
}

func runCeilingOnce(sessionID, stateDir string, times int) ceilingRun {
	GinkgoHelper()

	path, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "enforce-cycle-ceiling.sh"))
	Expect(err).NotTo(HaveOccurred())

	var last ceilingRun
	for i := 0; i < times; i++ {
		body, err := json.Marshal(map[string]any{"session_id": sessionID})
		Expect(err).NotTo(HaveOccurred())

		cmd := exec.Command("bash", path)
		cmd.Stdin = bytes.NewReader(body)
		cmd.Env = append(os.Environ(), "COACH_CYCLE_STATE_DIR="+stateDir)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		Expect(cmd.Run()).To(Succeed())

		last = ceilingRun{stderr: stderr.String()}
		if out := strings.TrimSpace(stdout.String()); out != "" {
			var payload struct {
				Decision string `json:"decision"`
				Reason   string `json:"reason"`
			}
			Expect(json.Unmarshal([]byte(out), &payload)).To(Succeed())
			last.blocked = payload.Decision == "block"
			last.reason = payload.Reason
		}
	}
	return last
}

// The ceiling is a hard block, and a hard block lands wherever the run happens
// to be -- most likely on uncommitted work, in a container that gets reclaimed.
// The arithmetic makes that likelier than it sounds: six tasks at three cycles
// each is eighteen reviews before a single integration round or repair attempt,
// against a ceiling of twenty-four.
//
// So there is a warning before the wall. It does not block; it tells the
// orchestrator to stop cleanly while it still can, which is the difference
// between a typed stop with committed work and a run that dies holding it.
var _ = Describe("reviewer cycle soft budget", func() {
	var stateDir string

	BeforeEach(func() { stateDir = GinkgoT().TempDir() })

	When("the run is well inside its budget", func() {
		It("says nothing at all", func() {
			run := runCeilingOnce("quiet", stateDir, 5)
			Expect(run.blocked).To(BeFalse())
			Expect(run.stderr).To(BeEmpty(),
				"a warning on every review is noise the orchestrator learns to ignore")
		})
	})

	When("the run approaches the ceiling", func() {
		It("warns without blocking, and says how much is left", func() {
			run := runCeilingOnce("approaching", stateDir, 21)
			Expect(run.blocked).To(BeFalse(),
				"the soft budget must not block; blocking early would make it the ceiling")
			Expect(run.stderr).NotTo(BeEmpty(),
				"a budget nobody is told about cannot change what the orchestrator does")
			Expect(run.stderr).To(MatchRegexp(`(?i)\b(21|3|remaining|left)\b`),
				"a bare 'nearly done' gives the orchestrator nothing to decide with")
			Expect(run.stderr).To(MatchRegexp(`(?i)stop|commit|wrap`),
				"the warning has to name the action, or it is just an observation")
		})
	})

	When("the run crosses the ceiling anyway", func() {
		It("still blocks", func() {
			run := runCeilingOnce("over", stateDir, 25)
			Expect(run.blocked).To(BeTrue(),
				"the soft budget is a warning in front of the ceiling, not a replacement for it")
			Expect(run.reason).To(ContainSubstring("repeated-finding"))
		})
	})

	// The counter lives in a file any agent holding Bash can write. That is a
	// deliberate limitation, not an oversight -- it is the same observation that
	// withdrew the validation manifest as gate authority -- so the ceiling is a
	// runaway-cost backstop, not an adversarial control. The script has to say
	// so, or a future reader will treat it as a security boundary.
	It("records that it is a cost backstop, not an adversarial control", func() {
		body, err := os.ReadFile(filepath.Join("..", "..", ".claude", "hooks", "enforce-cycle-ceiling.sh"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(MatchRegexp(`(?si)(not an adversarial|cannot stop a determined|any agent.{0,40}(write|reach)|runaway-cost)`),
			"an agent with Bash can reset this counter; a reader who assumes otherwise will lean on it")
	})
})
