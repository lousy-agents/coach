package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

// HoldsCommittedPolicyConverted is whether the file at repoPath in the worktree of dir is the blob committed at revision
// once git's own conversions are applied. A checkout that converted line endings is clean to git status, so a scan that
// reads it back as the committed policy has not read an edit. Anything that cannot be answered is taken to differ.
func HoldsCommittedPolicyConverted(dir, revision, repoPath string) bool {
	run := NewGitRunner()
	root, err := gitrepo.RepositoryRootWith(run, dir)
	if err != nil {
		return false
	}
	committed, found, err := gitrepo.BlobEntryAtRevision(run, dir, revision, repoPath)
	if err != nil || !found || !isRegularFileMode(committed.Mode) {
		return false
	}
	return holdsTheCommittedBlobConverted(run, root, repoPath, committed.SHA)
}
