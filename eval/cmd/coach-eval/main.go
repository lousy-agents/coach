// Command coach-eval runs Coach's evaluation tasks (docs/evaluation/agentic-eval-design.md).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const exitTaskUnimplemented = 2

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "compare" {
		return evalharness.Compare(stateHome(), args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "generate" {
		return evalharness.Generate(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "coach-eval: no evaluation task is implemented yet")
	return exitTaskUnimplemented
}

func stateHome() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}
