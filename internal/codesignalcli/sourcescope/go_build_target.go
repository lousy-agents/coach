package sourcescope

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func runCommand(dir, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil && stderr.Len() > 0 {
		return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, err
}

// snapshotBuildTarget preserves the meaning of relative package patterns
// supplied from a subdirectory while making them point at the HEAD snapshot.
func snapshotBuildTarget(target, repositoryRoot, invocationDir, snapshotDir string) (string, error) {
	resolvedInvocationDir, err := filepath.EvalSymlinks(invocationDir)
	if err != nil {
		return "", fmt.Errorf("resolving invocation directory: %w", err)
	}
	if filepath.IsAbs(target) {
		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			return "", fmt.Errorf("resolving build target: %w", err)
		}
		rel, err := filepath.Rel(repositoryRoot, resolvedTarget)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("build target %q is outside the repository", target)
		}
		return filepath.Join(snapshotDir, rel), nil
	}
	if !strings.HasPrefix(target, ".") {
		return target, nil
	}
	relDir, err := filepath.Rel(repositoryRoot, resolvedInvocationDir)
	if err != nil {
		return "", fmt.Errorf("resolving build target: %w", err)
	}
	return filepath.Join(snapshotDir, relDir, target), nil
}
