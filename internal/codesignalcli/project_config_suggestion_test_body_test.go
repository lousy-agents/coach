package codesignalcli

import (
	"fmt"
	"io/fs"

	"strconv"
	"strings"
	"testing"
)

func body_projectConfigSuggestionTest_plainErrorEmbeddingAKnownAbsolutePathIsStripped_24(t *testing.T, absoluteDir string) {
	err := fmt.Errorf("coach codesignal: %s is not inside a Git worktree", absoluteDir)
	got := snapshotUnavailableMessage("resolve HEAD", err, absoluteDir)
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the absolute path", err, got)
	}
	if !strings.Contains(got, "resolve HEAD") {
		t.Errorf("snapshotUnavailableMessage(%q) = %q, want it to name the failing operation", err, got)
	}
}

func body_projectConfigSuggestionTest_fsPathErrorIsUnwrappedToItsErrnoDiscardingPathEn_35(t *testing.T, absoluteDir string) {
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

func body_projectConfigSuggestionTest_gitLsTreeErrorNamingTheResolvedRepositoryRootIsS_47(t *testing.T, absoluteDir string) {
	err := fmt.Errorf("coach: git ls-tree failed for revision %q in %q: exit status 128: fatal: not a tree object", "deadbeef", absoluteDir)
	got := snapshotUnavailableMessage("read the HEAD snapshot", err, absoluteDir)
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the absolute repository root", err, got)
	}
}

func body_projectConfigSuggestionTest_snapshotListErrorWhoseWrappedGitStderrEmbedsAKno_55(t *testing.T, absoluteDir string) {

	gitErr := fmt.Errorf("exit status 128: fatal: cannot change to '%s': No such file or directory", absoluteDir)
	listErr := &snapshotListError{revision: "deadbeef", dir: absoluteDir, err: gitErr}
	got := snapshotUnavailableMessage("read the HEAD snapshot", listErr, absoluteDir)
	if strings.Contains(got, absoluteDir) {
		t.Fatalf("snapshotUnavailableMessage(%v) = %q, still contains the absolute repository root embedded in git's own stderr", listErr, got)
	}
}

func body_projectConfigSuggestionTest_gitLsTreeErrorWhoseQRenderingNeededEscapingIsStr_65(t *testing.T) {
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
