package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// UncommittedPolicyState is what is true of a --project-config policy that is in the working tree and that the analyzed
// revision does not hold: what the scan says, the stable word for it, and whether the index holds work for the path.
type UncommittedPolicyState struct {
	Message string
	Cause   projectreadiness.PolicyCause
	// HeldByTheIndex is whether the index holds a staged copy of the policy or an unresolved conflict for it. The file in
	// the working tree is then not the whole of what is held at the path, so a report that read the file has not read what
	// the index holds, and the path stays among the changes it did not read.
	HeldByTheIndex bool
}

// UncommittedPolicyInIndex is UncommittedPolicy for a policy that is on disk, with what the index holds for the path
// taken into account. A conflict the index records for it is told as one and not as a file to commit, because git
// refuses to commit while a path is unmerged, and its cause is merge_conflict, as it is when the file is gone. A staged
// copy keeps the words for a file that is not committed, because committing it is the way out, and is held by the
// index all the same. An entry that holds no content (git add --intent-to-add) is nothing the index holds: it is not an
// edit, and the file on disk is the policy. The error is a question git could not answer, with the message and the
// cause that give no advice, for a caller that reports a failure of git apart from a verdict about the policy.
func UncommittedPolicyInIndex(run gitrepo.Runner, dir, revision, repoPath string) (UncommittedPolicyState, error) {
	message, cause, unanswered := UncommittedPolicy(run, dir, revision, repoPath)
	if unanswered != nil {
		return UncommittedPolicyState{Message: message, Cause: cause}, unanswered
	}
	indexed, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		message, cause, unanswered = unansweredUncommitted(repoPath, revision, newGitFailure("what the index holds for it", err))
		return UncommittedPolicyState{Message: message, Cause: cause}, unanswered
	}
	return stateAmong(withoutEmptyEntries(indexed.Entries), UncommittedPolicyState{Message: message, Cause: cause}, revision, repoPath), nil
}

// stateAmong is the state of an uncommitted policy given the entries the index holds for it that carry content.
func stateAmong(entries []gitrepo.IndexEntry, uncommitted UncommittedPolicyState, revision, repoPath string) UncommittedPolicyState {
	switch {
	case holdsUnmergedEntry(entries):
		return UncommittedPolicyState{Message: unmergedOnDiskMessage(repoPath, revision), Cause: projectreadiness.CauseMergeConflict, HeldByTheIndex: true}
	case len(entries) > 0:
		uncommitted.HeldByTheIndex = true
	}
	return uncommitted
}

// unmergedOnDiskMessage is the scan's message for a policy on disk that the index holds as an unresolved conflict. The
// file may hold conflict markers or one side's version; either way it is resolved, marked resolved, and then committed.
func unmergedOnDiskMessage(repoPath, revision string) string {
	return fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but the index holds an unresolved merge conflict for it, so it is not committed at revision %q (project_config_invalid): resolve the conflict in it, mark it resolved, and commit it so it is readable at the analyzed revision", repoPath, revision)
}
