package evalharness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

type hookFailure struct{ stage, detail string }

func newHookChecker(coach string, programs []healthProgram) (*hookChecker, error) {
	checker := &hookChecker{plans: map[string]probePlan{}}
	wantsTypeScript := false
	for _, program := range programs {
		plan, err := planProbe(program.Path)
		if err != nil {
			return nil, err
		}
		checker.plans[program.Path] = plan
		wantsTypeScript = wantsTypeScript || plan.language == "typescript"
	}
	if wantsTypeScript {
		workingDir, _ := os.Getwd()
		tools, err := evalgen.ResolveTypeScriptTools(workingDir)
		if err != nil {
			return nil, err
		}
		checker.tools = tools
	}
	dir, err := os.MkdirTemp("", "coach-eval-hook-health-bin")
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(coach)
	if err != nil {
		return nil, err
	}
	if err := os.Symlink(absolute, filepath.Join(dir, "coach")); err != nil {
		return nil, err
	}
	checker.coachDir = dir
	return checker, nil
}

func (c *hookChecker) checkAll(programs []healthProgram, stdout, stderr io.Writer) int {
	defer os.RemoveAll(c.coachDir)
	status := HookHealthOK
	for _, program := range programs {
		failure, err := c.check(program)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "coach-eval hook-health: %v\n", err)
			return HookHealthMissing
		case failure != nil:
			fmt.Fprintf(stderr, "FAIL %s %s: %s\n", program.ProgramID, failure.stage, failure.detail)
			status = HookHealthFailed
		default:
			fmt.Fprintf(stdout, "ok %s\n", program.ProgramID)
		}
	}
	return status
}

// check copies the program, commits it, then runs the health check and the probe on the copy.
func (c *hookChecker) check(program healthProgram) (*hookFailure, error) {
	work, err := os.MkdirTemp("", "coach-eval-hook-health")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	plan := c.plans[program.Path]
	if err := c.prepare(program.Path, work, plan); err != nil {
		return nil, err
	}
	if failure := c.health(work); failure != nil {
		return failure, nil
	}
	return c.probe(work, plan), nil
}

func (c *hookChecker) prepare(source, work string, plan probePlan) error {
	if err := copyProgram(source, work); err != nil {
		return err
	}
	if plan.language == "typescript" {
		if err := provisionCompiler(c.tools.TSC, work); err != nil {
			return err
		}
	}
	for _, step := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.name=coach-eval", "-c", "user.email=coach-eval@example.invalid", "commit", "-q", "-m", "initial"},
	} {
		if output, err := run(work, nil, "", "git", step...); err != nil {
			return fmt.Errorf("git %s in the copy of %s: %v: %s", step[0], source, err, output)
		}
	}
	return nil
}

// health requires a clean review on the unmodified program: it reads the policy as HEAD has it and scans the whole
// worktree, and the hook then exits 0.
func (c *hookChecker) health(work string) *hookFailure {
	if detail := c.reviewProblem(work); detail != "" {
		return &hookFailure{stageHealth, detail}
	}
	exit, output := c.hook(work)
	if exit != 0 {
		return &hookFailure{stageHealth, fmt.Sprintf("the hook blocked the unmodified program (exit %d): %s", exit, strings.TrimSpace(output))}
	}
	return nil
}

// probe adds the planned file, uncommitted, and requires the hook to block with the layer violation at that path.
func (c *hookChecker) probe(work string, plan probePlan) *hookFailure {
	target := filepath.Join(work, filepath.FromSlash(plan.rel))
	if err := os.WriteFile(target, []byte(plan.content), 0o644); err != nil {
		return &hookFailure{stageProbe, err.Error()}
	}
	exit, output := c.hook(work)
	switch {
	case exit != 2:
		return &hookFailure{stageProbe, fmt.Sprintf("the hook exited %d on %s, which imports across a forbidden edge, and should block (exit 2): %s", exit, plan.rel, strings.TrimSpace(output))}
	case !strings.Contains(output, layerRule) || !strings.Contains(output, plan.rel):
		return &hookFailure{stageProbe, fmt.Sprintf("the hook blocked without naming %s at %s: %s", layerRule, plan.rel, strings.TrimSpace(output))}
	}
	return nil
}

func (c *hookChecker) hook(work string) (int, string) {
	env := []string{"PATH=" + c.coachDir + string(os.PathListSeparator) + os.Getenv("PATH"), "CLAUDE_PROJECT_DIR=" + work}
	output, err := run(work, env, hookPayload, "sh", hookPath)
	return exitCodeOf(err), output
}

// reviewProblem reads the report the hook reads, and says what makes a program that should be clean not clean.
func (c *hookChecker) reviewProblem(work string) string {
	env := []string{"PATH=" + c.coachDir + string(os.PathListSeparator) + os.Getenv("PATH")}
	output, err := run(work, env, "", filepath.Join(c.coachDir, "coach"), "codesignal", "--worktree", "--no-interactive", "--fail-on-incomplete-coverage", "--format", "json")
	if err != nil {
		return fmt.Sprintf("the review did not run cleanly (exit %d): %s", exitCodeOf(err), strings.TrimSpace(output))
	}
	var report struct {
		Scope struct {
			Policy *struct {
				MatchesHead *bool `json:"matches_head"`
			} `json:"policy"`
		} `json:"scope"`
		Diagnostics []struct {
			Kind string `json:"kind"`
		} `json:"diagnostics"`
	}
	if json.Unmarshal([]byte(output), &report) != nil {
		return "the review report is not JSON: " + strings.TrimSpace(output)
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind == "worktree_scan_incomplete" {
			return "the review reports worktree_scan_incomplete"
		}
	}
	if policy := report.Scope.Policy; policy == nil || policy.MatchesHead == nil || !*policy.MatchesHead {
		return "scope.policy.matches_head is not true on the unmodified program"
	}
	return ""
}

func run(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if err == nil {
		return 0
	}
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
