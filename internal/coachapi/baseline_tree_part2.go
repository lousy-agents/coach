package coachapi

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/githubingest"

	"io/fs"

	"path/filepath"
)

func (w *localFixtureTreeWalker) visit(path string, d fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}

	if d.Type()&fs.ModeSymlink != 0 {
		return nil
	}
	if d.IsDir() {
		return nil
	}
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if !supportedBaselinePath(rel) {
		return nil
	}
	fi, err := d.Info()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return nil
	}
	size := int(fi.Size())
	if w.opts.MaxFiles > 0 && len(w.out)+1 > w.opts.MaxFiles {
		return fmt.Errorf("coachapi: local fixture tree exceeds the configured file-count budget of %d: %w", w.opts.MaxFiles, githubingest.ErrTooLarge)
	}
	newTotal := w.totalBytes + int64(size)
	if w.opts.MaxTotalBytes > 0 && newTotal > w.opts.MaxTotalBytes {
		return fmt.Errorf("coachapi: local fixture tree exceeds the configured byte budget of %d bytes: %w", w.opts.MaxTotalBytes, githubingest.ErrTooLarge)
	}
	w.totalBytes = newTotal
	w.out = append(w.out, BaselineFileEntry{Path: rel, Size: size})
	return nil
}
func (s *ResolvingGitHubBaselineTreeSource) ReadFile(ctx context.Context, owner, repo, ref, path string) ([]byte, string, error) {
	reader, err := s.readerFor(ctx, owner, repo)
	if err != nil {
		return nil, "", err
	}
	return (&GitHubBaselineTreeSource{Reader: reader}).ReadFile(ctx, owner, repo, ref, path)
}
