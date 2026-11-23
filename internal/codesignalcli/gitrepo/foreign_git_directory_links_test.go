package gitrepo

import (
	"errors"
	"path/filepath"
	"testing"
)

// What is left to git or to the scan's own reads passes without a further question.
func TestRefuseForeignGitDirectoryLetsThroughWhatGitLinks(t *testing.T) {
	t.Setenv("GIT_WORK_TREE", "")
	root := t.TempDir()
	scan, gitDir := filepath.Join(root, "scan"), filepath.Join(root, "other", ".git")
	separateGitDir := filepath.Join(root, "other", "store.git")
	linkedGitDir := filepath.Join(gitDir, "worktrees", "scan")
	notCommonGitDir := filepath.Join(root, "elsewhere", ".git")
	makeDirectories(t, filepath.Join(scan, ".git"), gitDir, separateGitDir, linkedGitDir, notCommonGitDir)
	cases := map[string]answers{
		"the directory is not inside a work tree": {
			insideQuestion: {output: "false\n"},
		},
		"git says the directory is not a repository": {
			insideQuestion: {err: gitWaitError(exitStatus128, []byte("fatal: not a git repository (or any of the parent directories): .git\n"))},
		},
		"the git directory is a linked worktree's": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: linkedGitDir + "\n" + gitDir + "\n"},
		},
		"the git directory is named .git but is not its repository's main one": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: notCommonGitDir + "\n" + gitDir + "\n"},
		},
		"the common directory is printed relative to the directory git ran in": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: filepath.Join(scan, ".git") + "\n.git\n"},
		},
		"the main git directory is not named .git": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: separateGitDir + "\n" + separateGitDir + "\n"},
		},
		"core.worktree names the work tree": {
			insideQuestion:  {output: "true\n"},
			layoutQuestion:  {output: gitDir + "\n" + gitDir + "\n"},
			worktreeSetting: {output: scan + "\n"},
		},
	}
	for name, git := range cases {
		t.Run(name, func(t *testing.T) { assertLetThrough(t, git, scan) })
	}
}

// GIT_WORK_TREE excuses nothing: with it naming the scanned directory, a main git directory named .git that does
// not hold that directory is refused, and a git directory under another name is let through as it is without it.
func TestRefuseForeignGitDirectoryJudgesTheLayoutWhenTheCallerNamesAWorkTree(t *testing.T) {
	root := t.TempDir()
	scan, gitDir := filepath.Join(root, "scan"), filepath.Join(root, "other", ".git")
	separateGitDir := filepath.Join(root, "other", "store.git")
	makeDirectories(t, scan, gitDir, separateGitDir)
	t.Setenv("GIT_WORK_TREE", scan)

	t.Run("a main git directory named .git outside the work tree is refused", func(t *testing.T) {
		foreign := answers{
			insideQuestion:  {output: "true\n"},
			layoutQuestion:  {output: gitDir + "\n" + gitDir + "\n"},
			worktreeSetting: {err: exitStatusOne},
		}

		err := RefuseForeignGitDirectory(foreign.run, scan)

		var refusal *ForeignGitDirectoryError
		if !errors.As(err, &refusal) {
			t.Errorf("RefuseForeignGitDirectory = %v; want a ForeignGitDirectoryError", err)
		}
	})
	t.Run("a main git directory not named .git is let through", func(t *testing.T) {
		separate := answers{
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: separateGitDir + "\n" + separateGitDir + "\n"},
		}

		assertLetThrough(t, separate, scan)
	})
}

func assertLetThrough(t *testing.T, git answers, scan string) {
	t.Helper()
	if err := RefuseForeignGitDirectory(git.run, scan); err != nil {
		t.Errorf("RefuseForeignGitDirectory = %v; want nil", err)
	}
}
