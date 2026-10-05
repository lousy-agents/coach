package baseline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

func (s *LocalFixtureTreeSource) ReadFile(_ context.Context, _, _, _, path string) ([]byte, string, error) {
	if s == nil || s.Root == "" {
		return nil, "", fmt.Errorf("coachapi: local fixture path is not configured")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, "", fmt.Errorf("coachapi: resolving smoke fixture path: %w", err)
	}
	// filepath.Join drops root if path is absolute; Clean+Rel enforce containment.
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("coachapi: path %q escapes smoke fixture root: %w", path, githubingest.ErrNotFound)
	}
	// Lstat: never follow a symlink under root that points outside.
	fi, err := os.Lstat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("coachapi: fixture file %q: %w", path, githubingest.ErrNotFound)
		}
		return nil, "", err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("coachapi: fixture path %q is a symlink: %w", path, githubingest.ErrUnsupportedContent)
	}
	if !fi.Mode().IsRegular() {
		return nil, "", fmt.Errorf("coachapi: fixture path %q is not a regular file: %w", path, githubingest.ErrUnsupportedContent)
	}
	content, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("coachapi: fixture file %q: %w", path, githubingest.ErrNotFound)
		}
		return nil, "", err
	}
	if len(content) == 0 {
		return nil, "", fmt.Errorf("coachapi: fixture file %q: %w", path, githubingest.ErrEmptyContent)
	}
	return content, "local-fixture", nil
}
