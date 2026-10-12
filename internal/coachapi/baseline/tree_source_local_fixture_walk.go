package baseline

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// localFixtureTreeWalker accumulates FileEntry rows across one
// filepath.WalkDir traversal, enforcing opts' file-count and byte budgets
// as it goes so a WalkDir over-budget stops (via a returned error) instead
// of building the whole tree first.
type localFixtureTreeWalker struct {
	root       string
	opts       ListOptions
	out        []FileEntry
	totalBytes int64
}

func (w *localFixtureTreeWalker) visit(path string, d fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	// Do not follow symlinks (githubingest Contents parity; blocks root escape).
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
	w.out = append(w.out, FileEntry{Path: rel, Size: size})
	return nil
}
