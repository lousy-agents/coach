package projectreadiness

// PolicyCause is why a policy gap that no remedy fixes has no way out, as a word a
// caller can match on. A gap with a remedy (author a new policy, restore the
// committed one) names what to do and carries no cause. The values are stable: the
// readiness result carries them on the gap, and the text prints the same word.
type PolicyCause string

const (
	// CauseNotCommitted is a policy file on disk that the analyzed revision does not
	// hold, or one inside a submodule, which the superproject cannot commit.
	CauseNotCommitted PolicyCause = "not_committed"
	// CauseParentNotDirectory is a file where a parent directory of the policy should be.
	CauseParentNotDirectory PolicyCause = "parent_not_directory"
	// CauseStagedEdit is a committed policy whose index entry holds an edit.
	CauseStagedEdit PolicyCause = "staged_edit"
	// CauseStagedCopy is a policy that is not committed and whose index holds a copy.
	CauseStagedCopy PolicyCause = "staged_copy"
	// CauseStagedNotAFile is a policy that is not committed and is staged as a link
	// or another entry that is not a regular file.
	CauseStagedNotAFile PolicyCause = "staged_not_a_file"
	// CauseMergeConflict is an unmerged index entry for the policy.
	CauseMergeConflict PolicyCause = "merge_conflict"
	// CauseSkipWorktree is a skip-worktree mark on the policy's index entry.
	CauseSkipWorktree PolicyCause = "skip_worktree"
	// CauseSparseCheckout is a sparse checkout that leaves the policy out.
	CauseSparseCheckout PolicyCause = "sparse_checkout"
	// CauseCommittedAsLink is a policy committed as a symbolic link.
	CauseCommittedAsLink PolicyCause = "committed_as_link"
	// CauseDirectory is a path the index holds files under, so it is a directory.
	CauseDirectory PolicyCause = "directory"
	// CauseIndexLocked is an index that another git process holds locked.
	CauseIndexLocked PolicyCause = "index_locked"
	// CauseGitUnreadable is a question about the policy that git could not answer.
	CauseGitUnreadable PolicyCause = "git_unreadable"
	// CauseUnusableOnDisk is a path on disk that cannot be a policy: not a regular
	// file, a link, a name that differs by case, or unreadable.
	CauseUnusableOnDisk PolicyCause = "unusable_on_disk"
	// CauseUndetermined is a state Coach has no word for.
	CauseUndetermined PolicyCause = "undetermined"
)
