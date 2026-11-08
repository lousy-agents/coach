package gitrepo

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// aCloneWhoseIndexNamesABlobThatIsNotLocal is a blob:none clone with its checkout, whose index then names a blob
// that nothing reachable from HEAD names and that is not local: a branch is fetched, one path is reset from it, and a
// tracked file is dropped from the index, so that the staged deletion and the staged addition are a pair for rename
// detection to compare. The guard lists the objects reachable from the revisions the scan reads, and HEAD lacks none.
func aCloneWhoseIndexNamesABlobThatIsNotLocal(t *testing.T) string {
	t.Helper()
	source := twoCommitSource(t)
	trunk := strings.TrimSpace(gitfixture.Git(t, source, "branch", "--show-current"))
	gitfixture.Git(t, source, "checkout", "--quiet", "-b", "feature")
	gitfixture.CommitFile(t, source, "pkg/b.go", "package b\n\nfunc B() {}\n")
	gitfixture.Git(t, source, "checkout", "--quiet", trunk)
	bare := filepath.Join(t.TempDir(), "bare.git")
	gitOutput(t, source, "clone", "--bare", "--quiet", source, bare)
	gitOutput(t, bare, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(t.TempDir(), "part")
	gitOutput(t, t.TempDir(), "clone", "--quiet", "--filter=blob:none", "file://"+bare, clone)
	gitOutput(t, clone, "fetch", "--quiet", "--filter=blob:none", "origin", "feature")
	gitOutput(t, clone, "reset", "--quiet", "FETCH_HEAD", "--", "pkg/b.go")
	gitOutput(t, clone, "rm", "--quiet", "--cached", "a.go")
	gitOutput(t, clone, "remote", "set-url", "origin", "ssh://example.invalid/repo.git")
	return clone
}

// `git status` compares the staged deletion with the staged addition to find a rename, and that reads the blob of
// each, so it fetches the one the index names and the repository lacks, on a git that ignores the instruction not to
// fetch. The guard cannot see it, since it lists what the revisions reach. So the status does not look for renames.
func TestTheStatusForAGitBefore244DoesNotFetchABlobOnlyTheIndexNames(t *testing.T) {
	clone := aCloneWhoseIndexNamesABlobThatIsNotLocal(t)
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, clone)

	entries, err := worktreeStatusOf(gitBefore244, clone, false, olderThanTheInstruction)

	if err != nil || transportRan(marker) || objectsIn(t, clone) != before {
		t.Errorf("worktreeStatusOf = %v, %v; want the status to go on, with no transport run and no object written", entries, err)
	}
	if !statusLists(entries, "D ", "a.go") || !statusLists(entries, "AD", "pkg/b.go") {
		t.Errorf("worktreeStatusOf listed %+v; want the staged deletion of a.go and the staged addition of pkg/b.go, each as its own path", entries)
	}
}

// The canary is live: the status that looks for renames fetches that blob.
func TestTheStatusThatLooksForRenamesFetchesABlobOnlyTheIndexNames(t *testing.T) {
	clone := aCloneWhoseIndexNamesABlobThatIsNotLocal(t)
	asGitBefore244(t)
	marker := sshMarker(t)

	_, _ = gitBefore244(clone, "status", "--porcelain=v1", "--untracked-files=all", "-z")

	if !transportRan(marker) {
		t.Errorf("the transport did not run: a test that asserts it did not proves nothing")
	}
}

func statusLists(entries []WorktreeEntry, code, path string) bool {
	return slices.Contains(entries, WorktreeEntry{Code: code, Path: path})
}
