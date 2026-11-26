package evalharness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

const emptyGremlinsReport = `{"files":[]}`

// runGremlins unleashes Gremlins on a tree and returns its JSON report. Gremlins v0.6.0 writes no report when
// the tree has nothing to mutate (observed on branch-free programs, and relied on rather than documented by the
// tool), so a missing report is an empty one; any other failure to read it is an error, never a vacuous pass.
func runGremlins(gremlins Gremlins, tree map[string][]byte) ([]byte, error) {
	directory, err := os.MkdirTemp("", "coach-eval-mutation")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	if err := evalgen.WriteTree(directory, tree); err != nil {
		return nil, err
	}
	reportPath := filepath.Join(directory, ".gremlins-report.json")
	if output, err := unleash(gremlins, directory, reportPath); err != nil {
		return nil, fmt.Errorf("gremlins: %v\n%s", err, output)
	}
	report, err := os.ReadFile(reportPath)
	if os.IsNotExist(err) {
		return []byte(emptyGremlinsReport), nil
	}
	return report, err
}

// DefaultGremlinsLimit is how long one Gremlins run may take. A run mutates each covered branch and runs the suite,
// so a large program takes minutes; a run that outlasts this is hung.
const DefaultGremlinsLimit = 30 * time.Minute

// gremlinsOutputGrace is how long a process that outlives Gremlins may keep the run's output open after the kill.
const gremlinsOutputGrace = 2 * time.Second

// unleash runs Gremlins in its own process group and kills the group when the time limit passes, so a hung run, or
// one that left a child behind, cannot stall generation.
func unleash(gremlins Gremlins, directory, reportPath string) (string, error) {
	limit := gremlins.Limit
	if limit <= 0 {
		limit = DefaultGremlinsLimit
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	var output bytes.Buffer
	command := exec.CommandContext(ctx, gremlins.Binary, "unleash", "--output", reportPath)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off", "GOTOOLCHAIN=local")
	command.Stdout, command.Stderr = &output, &output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = gremlinsOutputGrace
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output.String(), fmt.Errorf("did not finish within the time limit of %s", limit)
	}
	return output.String(), err
}

// mutationCache keeps a Gremlins report by the content of what it judged and the Gremlins version, so the same
// program, regenerated for another reason, does not pay for the same run twice.
type mutationCache struct{ directory string }

func (c mutationCache) key(tree map[string][]byte, version string) string {
	sum := sha256.Sum256([]byte(evalgen.TreeID(tree) + "|gremlins=" + version))
	return hex.EncodeToString(sum[:])
}

// report returns the cached report for the tree, or runs Gremlins and keeps the result.
func (c mutationCache) report(gremlins Gremlins, tree map[string][]byte) ([]byte, error) {
	path := filepath.Join(c.directory, c.key(tree, gremlins.Version)+".json")
	if cached, err := os.ReadFile(path); err == nil {
		return cached, nil
	}
	report, err := runGremlins(gremlins, tree)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.directory, 0o755); err != nil {
		return nil, err
	}
	return report, os.WriteFile(path, report, 0o644)
}
