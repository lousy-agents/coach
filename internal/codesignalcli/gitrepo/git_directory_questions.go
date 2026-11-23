package gitrepo

import (
	"fmt"
	"path/filepath"
	"strings"
)

// gitDirectoryLayout is where git says the git directory and its repository's common directory are.
type gitDirectoryLayout struct{ gitDir, commonDir string }

// directoryIsInsideWorkTree is false for a directory git does not call a work tree, including one it says is not a
// repository at all; only a bound the run hit is an error.
func directoryIsInsideWorkTree(run Runner, dir string) (bool, error) {
	inside, err := run(dir, "rev-parse", "--is-inside-work-tree")
	if bound := BoundFailure(err); bound != nil {
		return false, bound
	}
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(inside)) == "true", nil
}

// isNamedMainGitDirectory is whether the git directory is its repository's own and is named .git, the only kind of
// git directory whose parent is where its work tree is expected to be.
func (l gitDirectoryLayout) isNamedMainGitDirectory() (bool, error) {
	isMain, err := sameDirectory(l.gitDir, l.commonDir)
	return isMain && filepath.Base(l.gitDir) == ".git", err
}

func readGitDirectoryLayout(run Runner, dir string) (gitDirectoryLayout, error) {
	output, err := run(dir, "rev-parse", "--absolute-git-dir", "--git-common-dir")
	if err != nil {
		return gitDirectoryLayout{}, foreignGitDirectoryUnanswered(err)
	}
	paths := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(paths) != 2 {
		return gitDirectoryLayout{}, foreignGitDirectoryUnanswered(fmt.Errorf("git printed %d lines, want 2", len(paths)))
	}
	gitDir, commonDir := paths[0], paths[1]
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	return gitDirectoryLayout{gitDir: gitDir, commonDir: commonDir}, nil
}

func namesItsWorkTree(run Runner, dir string) (bool, error) {
	_, err := run(dir, "config", "--get", "core.worktree")
	if err == nil {
		return true, nil
	}
	if !ConfigHasNoValue(err) {
		return false, foreignGitDirectoryUnanswered(err)
	}
	return false, nil
}
