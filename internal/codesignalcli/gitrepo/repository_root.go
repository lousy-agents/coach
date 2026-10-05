package gitrepo

import (
	"fmt"
	"path/filepath"
	"strings"
)

func RepositoryRoot(dir string) (string, error) {
	output, err := Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("determining repository root: %w", err)
	}
	root := strings.TrimSpace(output)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolving repository root: %w", err)
	}
	return resolved, nil
}
