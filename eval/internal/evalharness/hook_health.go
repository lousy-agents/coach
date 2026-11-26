package evalharness

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Exit codes of `coach-eval hook-health` (design §10).
const (
	HookHealthOK      = 0
	HookHealthFailed  = 1
	HookHealthMissing = 2
)

// healthProgram is one line of `coach-eval generate` output: the program to check.
type healthProgram struct {
	ProgramID string `json:"program_id"`
	Path      string `json:"path"`
}

var errHookHealthInput = errors.New("missing or invalid input")

// HookHealth checks, for every program that `coach-eval generate` printed on stdin, that the self-review hook is
// healthy on the unmodified program and blocks a probe edit that adds a forbidden import. It prints "ok <program_id>"
// for a healthy program and "FAIL <program_id> <stage>: <detail>" on stderr for the others.
func HookHealth(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	coach, err := hookHealthSetup(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval hook-health: %v\n", err)
		return HookHealthMissing
	}
	programs, err := readHealthPrograms(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval hook-health: %v\n", err)
		return HookHealthMissing
	}
	checker, err := newHookChecker(coach, programs)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval hook-health: %v\n", err)
		return HookHealthMissing
	}
	return checker.checkAll(programs, stdout, stderr)
}

func hookHealthSetup(args []string, stderr io.Writer) (string, error) {
	flags := flag.NewFlagSet("coach-eval hook-health", flag.ContinueOnError)
	flags.SetOutput(stderr)
	coach := flags.String("coach", "", "the coach binary the hook runs")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if *coach == "" {
		return "", fmt.Errorf("%w: --coach names the coach binary the hook runs", errHookHealthInput)
	}
	if _, err := os.Stat(*coach); err != nil {
		return "", fmt.Errorf("the coach binary %s is not usable: %v", *coach, err)
	}
	for _, tool := range []string{"git", "jq", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			return "", fmt.Errorf("%s is not on PATH: the hook and its checks need it", tool)
		}
	}
	return *coach, nil
}
