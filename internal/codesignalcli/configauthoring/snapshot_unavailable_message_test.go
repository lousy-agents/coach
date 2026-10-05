package configauthoring

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/revisionfs"
)

// TestSnapshotUnavailableMessageNeverLeaksAbsolutePath pins
// snapshotUnavailableMessage's contract directly at the unit level: no
// absolute host filesystem path may survive into the returned message,
// regardless of whether the underlying error is a plain string that
// happens to embed a known absolute path (as gitrepo.resolveHEAD's
// "not inside a Git worktree" and revisionfs.New's "git ls-tree failed
// ... in %q" both are) or an *fs.PathError carrying an absolute path the
// caller never supplied (as filepath.EvalSymlinks' failure inside
// gitrepo.RepositoryRoot is).
func TestSnapshotUnavailableMessageNeverLeaksAbsolutePath(t *testing.T) {
	const absoluteDir = "/tmp/coach-acceptance-repo-123"

	t.Run("plain error embedding a known absolute path is stripped", func(t *testing.T) {
		plainErrorEmbeddingKnownAbsolutePathStripped(t, absoluteDir)
	})

	t.Run("fs.PathError is unwrapped to its errno, discarding Path entirely", func(t *testing.T) {
		fsPathErrorUnwrappedErrnoDiscardingPathEntirely(t, absoluteDir)
	})

	t.Run("git ls-tree error naming the resolved repository root is stripped", func(t *testing.T) {
		gitLsTreeErrorNamingResolvedRepositoryRoot(t, absoluteDir)
	})

	t.Run("snapshotListError whose wrapped git stderr embeds a known absolute path is stripped", func(t *testing.T) {
		snapshotListErrorWhoseWrappedGitStderrEmbedsKnownAbsolute(t, absoluteDir)
	})

	t.Run("git ls-tree error whose %q-rendering needed escaping is stripped in both raw and escaped form", gitLsTreeErrorWhoseQRenderingNeeded)
}

func plainErrorEmbeddingKnownAbsolutePathStripped(t *testing.T, absoluteDir string) {
	err := fmt.Errorf("coach codesignal: %s is not inside a Git worktree", absoluteDir)
	got := snapshotUnavailableMessage("resolve HEAD", err, absoluteDir)
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the absolute path", err, got)
	}
	if !strings.Contains(got, "resolve HEAD") {
		t.Errorf("snapshotUnavailableMessage(%q) = %q, want it to name the failing operation", err, got)
	}
}

func fsPathErrorUnwrappedErrnoDiscardingPathEntirely(t *testing.T, absoluteDir string) {
	pathErr := &fs.PathError{Op: "lstat", Path: absoluteDir, Err: fs.ErrNotExist}
	wrapped := fmt.Errorf("resolving repository root: %w", pathErr)
	got := snapshotUnavailableMessage("resolve the repository root", wrapped, "")
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%v) = %q, still contains the PathError's absolute path", wrapped, got)
	}
	if !strings.Contains(got, fs.ErrNotExist.Error()) {
		t.Errorf("snapshotUnavailableMessage(%v) = %q, want it to retain the errno-class reason %q", wrapped, got, fs.ErrNotExist.Error())
	}
}

func gitLsTreeErrorNamingResolvedRepositoryRoot(t *testing.T, absoluteDir string) {
	err := fmt.Errorf("coach: git ls-tree failed for revision %q in %q: exit status 128: fatal: not a tree object", "deadbeef", absoluteDir)
	got := snapshotUnavailableMessage("read the HEAD snapshot", err, absoluteDir)
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the absolute repository root", err, got)
	}
}

func snapshotListErrorWhoseWrappedGitStderrEmbedsKnownAbsolute(t *testing.T, absoluteDir string) {

	gitErr := fmt.Errorf("exit status 128: fatal: cannot change to '%s': No such file or directory", absoluteDir)
	listErr := &revisionfs.ListError{Revision: "deadbeef", Dir: absoluteDir, Err: gitErr}
	got := snapshotUnavailableMessage("read the HEAD snapshot", listErr, absoluteDir)
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%v) = %q, still contains the absolute repository root embedded in git's own stderr", listErr, got)
	}
}

func gitLsTreeErrorWhoseQRenderingNeeded(t *testing.T) {
	const quotableDir = `/tmp/coach-acceptance-repo-123"quote\dir`
	err := fmt.Errorf("coach: git ls-tree failed for revision %q in %q: exit status 128: fatal: not a tree object", "deadbeef", quotableDir)
	got := snapshotUnavailableMessage("read the HEAD snapshot", err, quotableDir)
	if strings.Contains(got, quotableDir) {
		t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the raw absolute repository root", err, got)
	}
	quoted := strconv.Quote(quotableDir)
	escaped := quoted[1 : len(quoted)-1]
	if strings.Contains(got, escaped) {
		t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the %%q-escaped absolute repository root %q", err, got, escaped)
	}
}
