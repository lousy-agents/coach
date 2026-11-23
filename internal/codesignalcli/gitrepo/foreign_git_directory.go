package gitrepo

import (
	"fmt"
	"path/filepath"
)

// RefuseForeignGitDirectory is nil unless git's repository is another repository's main git directory, named
// .git, that does not hold the directory the scan runs in. A scan reads the policy from the working tree and the
// history from the repository, so a GIT_DIR or a .git file that names a different repository would make one
// report out of two.
//
// Git itself links a work tree to a git directory in ways that need no check here: a linked worktree's git
// directory is not its repository's main one, a submodule's names its work tree in core.worktree, and a plain
// repository's sits in the directory that holds the work tree. That is judged by whether the scan's directory is
// inside the git directory's parent, not by git's own top level, because GIT_DIR alone makes git take the current
// directory for the top level of its work tree. GIT_WORK_TREE excuses nothing: it can name the scanned directory
// while GIT_DIR names another repository's own .git.
//
// Two limits are accepted. A gitfile pointing at a foreign git directory not named .git cannot be told apart from
// a legitimate separate one, and a separate git directory that is itself named .git but sits outside its work tree
// is refused unless core.worktree records the work tree.
//
// A directory that is not inside a work tree is left to the scan, whose own read says so. Any other question git
// leaves unanswered is a refusal: a repository that cannot be told apart from a foreign one is not read.
func RefuseForeignGitDirectory(run Runner, dir string) error {
	inside, err := directoryIsInsideWorkTree(run, dir)
	if err != nil {
		return err
	}
	if !inside {
		return nil
	}

	layout, err := readGitDirectoryLayout(run, dir)
	if err != nil {
		return err
	}
	candidate, err := layout.isNamedMainGitDirectory()
	if err != nil {
		return foreignGitDirectoryUnanswered(err)
	}
	if !candidate {
		return nil
	}
	held, err := directoryIsInside(dir, filepath.Dir(layout.gitDir))
	if err != nil {
		return foreignGitDirectoryUnanswered(err)
	}
	if held {
		return nil
	}

	namesWorkTree, err := namesItsWorkTree(run, dir)
	if err != nil {
		return err
	}
	if namesWorkTree {
		return nil
	}
	return &ForeignGitDirectoryError{GitDir: layout.gitDir}
}

func foreignGitDirectoryUnanswered(cause error) error {
	return &OperationalError{Message: fmt.Sprintf("could not tell whether the repository git reports is the one that holds this working tree (%s), so it is not read", TerminalSafe(cause.Error())), Cause: cause}
}
