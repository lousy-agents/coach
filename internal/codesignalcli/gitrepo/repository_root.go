package gitrepo

import (
	"fmt"
	"path/filepath"
	"strings"
)

func RepositoryRoot(dir string) (string, error) {
	return RepositoryRootWith(RunBytes, dir)
}

// RepositoryRootWith is RepositoryRoot through run, so a caller that holds its git
// reads to limits holds this one to them too.
func RepositoryRootWith(run Runner, dir string) (string, error) {
	output, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("determining repository root: %w", err)
	}
	root := strings.TrimSpace(string(output))
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolving repository root: %w", err)
	}
	return resolved, nil
}
