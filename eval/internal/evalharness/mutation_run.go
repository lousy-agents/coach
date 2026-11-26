package evalharness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

const emptyGremlinsReport = `{"files":[]}`

// runGremlins unleashes Gremlins on a tree and returns its JSON report. Gremlins writes no report when the
// tree has nothing to mutate, which is an empty report.
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
	command := exec.Command(gremlins.Binary, "unleash", "--output", reportPath)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gremlins: %v\n%s", err, output)
	}
	report, err := os.ReadFile(reportPath)
	if os.IsNotExist(err) {
		return []byte(emptyGremlinsReport), nil
	}
	return report, err
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
