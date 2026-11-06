package gitrepo

// CommittedWorktreeStatus lists the changed and untracked paths for a committed
// scan. It runs git with the caller's own environment and the repository's own
// configuration, as the committed scan always has; ReadWorktreeStatus is the read
// that a --worktree scan hardens against both.
func CommittedWorktreeStatus(dir string) ([]WorktreeEntry, error) {
	output, err := RunBytesBounded(dir, maxWorktreeStatusBytes, maxWorktreeStatusStderr, worktreeStatusTimeout,
		"status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return nil, err
	}
	return ParseWorktreeStatus(output), nil
}
