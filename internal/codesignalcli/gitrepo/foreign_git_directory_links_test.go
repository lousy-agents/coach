package gitrepo

import (
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

// With GIT_WORK_TREE set the guard asks git nothing, so a layout it would refuse is let through unasked.
func TestRefuseForeignGitDirectoryLetsThroughAWorkTreeTheCallerNames(t *testing.T) {
	root := t.TempDir()
	scan, gitDir := filepath.Join(root, "scan"), filepath.Join(root, "other", ".git")
	makeDirectories(t, scan, gitDir)
	foreign := answers{
		insideQuestion:  {output: "true\n"},
		layoutQuestion:  {output: gitDir + "\n" + gitDir + "\n"},
		worktreeSetting: {err: exitStatusOne},
	}
	t.Setenv("GIT_WORK_TREE", "")
	if err := RefuseForeignGitDirectory(foreign.run, scan); err == nil {
		t.Fatal("RefuseForeignGitDirectory = nil; the fixture must be refused without GIT_WORK_TREE")
	}

	t.Setenv("GIT_WORK_TREE", scan)

	assertLetThrough(t, foreign, scan)
}

func assertLetThrough(t *testing.T, git answers, scan string) {
	t.Helper()
	if err := RefuseForeignGitDirectory(git.run, scan); err != nil {
		t.Errorf("RefuseForeignGitDirectory = %v; want nil", err)
	}
}
