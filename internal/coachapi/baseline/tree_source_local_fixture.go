package baseline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LocalFixtureTreeSource walks an operator-configured directory tree.
// owner/repo/ref are ignored; the fixture root is the sole content source.
// Supported-language files under top-level dot paths (e.g. .github/) are kept
// for parity with GitHub Contents.
type LocalFixtureTreeSource struct {
	Root string
}

func (s *LocalFixtureTreeSource) ResolveCommitSHA(_ context.Context, _, _, _ string) (string, error) {
	if s == nil || s.Root == "" {
		return "", fmt.Errorf("coachapi: local fixture path is not configured")
	}
	return localFixtureCommitSHA, nil
}

func (s *LocalFixtureTreeSource) ListFiles(_ context.Context, _, _, _ string, opts ListOptions) ([]FileEntry, error) {
	if s == nil || s.Root == "" {
		return nil, fmt.Errorf("coachapi: local fixture path is not configured")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, fmt.Errorf("coachapi: resolving smoke fixture path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("coachapi: smoke fixture path %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("coachapi: smoke fixture path %q is not a directory", root)
	}

	walker := &localFixtureTreeWalker{root: root, opts: opts}
	if err := filepath.WalkDir(root, walker.visit); err != nil {
		return nil, err
	}
	out := walker.out
	if out == nil {
		out = []FileEntry{}
	}
	return out, nil
}
