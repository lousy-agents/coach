// Command coach-eval runs Coach's evaluation tasks (docs/evaluation/agentic-eval-design.md).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
	"github.com/lousy-agents/coach/eval/internal/evalhistory"
	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const exitTaskUnimplemented = 2

// commands are coach-eval's subcommands, each run with the arguments after its name.
var commands = map[string]func(args []string, stdout, stderr io.Writer) int{
	"compare": func(args []string, stdout, stderr io.Writer) int {
		return evalharness.Compare(stateHome(), args, stdout, stderr)
	},
	"census": func(args []string, stdout, stderr io.Writer) int {
		return evalcensus.Census(args, stdout, stderr, evalharness.CensusLine, time.Now)
	},
	"history": func(args []string, stdout, stderr io.Writer) int {
		return evalhistory.History(args, stdout, stderr, evalharness.HistoryLine, time.Now)
	},
	"generate": evalharness.Generate,
	"hook-health": func(args []string, stdout, stderr io.Writer) int {
		return evalharness.HookHealth(args, os.Stdin, stdout, stderr)
	},
	"antipatterns": evalharness.Antipatterns,
	"patterns":     evalpatterns.Patterns,
	"trend":        evalharness.Trend,
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		if command, ok := commands[args[0]]; ok {
			return command(args[1:], stdout, stderr)
		}
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
