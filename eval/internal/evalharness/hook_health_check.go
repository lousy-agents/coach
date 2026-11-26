package evalharness

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

const (
	stageHealth = "health"
	stageProbe  = "probe"
	hookPath    = "scripts/coach-self-review.sh"
	hookPayload = "{}"
	layerRule   = "architecture.layer_violation"
)

// hookChecker runs the hook the way the agent harness does: coach first on PATH, the project directory in the
// environment, a payload on standard input.
type hookChecker struct {
	coachDir string
	plans    map[string]probePlan
	tools    evalgen.TypeScriptTools
}

// hookFailure is the stage and detail of a failed check; the zero value is a check that passed.
type hookFailure struct{ stage, detail string }

func (f hookFailure) failed() bool { return f.stage != "" }

func (c *hookChecker) checkAll(programs []healthProgram, stdout, stderr io.Writer) int {
	defer os.RemoveAll(c.coachDir)
	status := HookHealthOK
	for _, program := range programs {
		failure, err := c.check(program)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "coach-eval hook-health: %v\n", err)
			return HookHealthMissing
		case failure.failed():
			fmt.Fprintf(stderr, "FAIL %s %s: %s\n", program.ProgramID, failure.stage, failure.detail)
			status = HookHealthFailed
		default:
			fmt.Fprintf(stdout, "ok %s\n", program.ProgramID)
		}
	}
	return status
}

// check copies the program, commits it, then runs the health check and the probe on the copy.
func (c *hookChecker) check(program healthProgram) (hookFailure, error) {
	work, err := os.MkdirTemp("", "coach-eval-hook-health")
	if err != nil {
		return hookFailure{}, err
	}
	defer os.RemoveAll(work)
	plan := c.plans[program.Path]
	if err := c.prepare(program.Path, work, plan); err != nil {
		return hookFailure{}, err
	}
	if failure := c.health(work); failure.failed() {
		return failure, nil
	}
	return c.probe(work, plan), nil
}

// health requires a clean review on the unmodified program: it reads the policy as HEAD has it and scans the whole
// worktree, and the hook then exits 0.
func (c *hookChecker) health(work string) hookFailure {
	if detail := c.reviewProblem(work); detail != "" {
		return hookFailure{stageHealth, detail}
	}
	exit, output := c.hook(work)
	if exit != 0 {
		return hookFailure{stageHealth, fmt.Sprintf("the hook blocked the unmodified program (exit %d): %s", exit, strings.TrimSpace(output))}
	}
	return hookFailure{}
}

// probe adds the planned file, uncommitted, and requires the hook to block with the layer violation at that path.
func (c *hookChecker) probe(work string, plan probePlan) hookFailure {
	target := filepath.Join(work, filepath.FromSlash(plan.rel))
	if err := os.WriteFile(target, []byte(plan.content), 0o644); err != nil {
		return hookFailure{stageProbe, err.Error()}
	}
	exit, output := c.hook(work)
	switch {
	case exit != 2:
		return hookFailure{stageProbe, fmt.Sprintf("the hook exited %d on %s, which imports across a forbidden edge, and should block (exit 2): %s", exit, plan.rel, strings.TrimSpace(output))}
	case !strings.Contains(output, layerRule) || !strings.Contains(output, plan.rel):
		return hookFailure{stageProbe, fmt.Sprintf("the hook blocked without naming %s at %s: %s", layerRule, plan.rel, strings.TrimSpace(output))}
	}
	return hookFailure{}
}

func (c *hookChecker) hook(work string) (int, string) {
	env := []string{"PATH=" + c.coachDir + string(os.PathListSeparator) + os.Getenv("PATH"), "CLAUDE_PROJECT_DIR=" + work}
	output, err := runIn(work, env, hookPayload, "sh", hookPath)
	return exitCodeOf(err), output
}
