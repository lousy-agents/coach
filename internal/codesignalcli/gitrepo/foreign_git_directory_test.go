package gitrepo

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

func assertRefusedAsUnanswered(t *testing.T, git answers, scan string) {
	t.Helper()
	err := RefuseForeignGitDirectory(git.run, scan)

	if err == nil {
		t.Fatal("RefuseForeignGitDirectory = nil; want a refusal")
	}
	var foreign *ForeignGitDirectoryError
	if errors.As(err, &foreign) {
		t.Errorf("RefuseForeignGitDirectory = %v; a question git left unanswered is not an answer that the repository is foreign", err)
	}
}

// A scan that sits outside the parent of a main git directory that names no work tree is the one case the guard
// refuses; every question git leaves unanswered on the way there is a refusal as well, and none is a pass.
func TestRefuseForeignGitDirectoryRefusesWhatItCannotTell(t *testing.T) {
	t.Setenv("GIT_WORK_TREE", "")
	root := t.TempDir()
	scan, other := filepath.Join(root, "scan"), filepath.Join(root, "other")
	gitDir := filepath.Join(other, ".git")
	makeDirectories(t, scan, gitDir)
	foreignLayout := gitDir + "\n" + gitDir + "\n"
	cases := map[string]answers{
		"git does not print two lines": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: gitDir + "\n"},
		},
		"git fails to name the git directory": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {err: exitStatus128},
		},
		"the git directory cannot be found on disk": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: filepath.Join(root, "gone") + "\n" + gitDir + "\n"},
		},
		"the common directory cannot be found on disk": {
			insideQuestion: {output: "true\n"},
			layoutQuestion: {output: gitDir + "\n" + filepath.Join(root, "gone") + "\n"},
		},
		"git fails to say whether core.worktree is set": {
			insideQuestion:  {output: "true\n"},
			layoutQuestion:  {output: foreignLayout},
			worktreeSetting: {err: exitStatus128},
		},
		"git runs out of time saying whether the directory is in a work tree": {
			insideQuestion: {err: &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}},
		},
		"git refuses a repository it does not trust": {
			insideQuestion: {err: gitWaitError(exitStatus128, []byte("fatal: detected dubious ownership in repository at '/other'\n"))},
		},
		"git fails with a status other than the one it uses for a missing repository": {
			insideQuestion: {err: gitWaitError(exitingWith(2), []byte("fatal: not a git repository\n"))},
		},
		"git fails to say anything about the directory": {
			insideQuestion: {err: exitStatus128},
		},
		"git cannot be run": {
			insideQuestion: {err: &exec.Error{Name: "git", Err: exec.ErrNotFound}},
		},
	}
	for name, git := range cases {
		t.Run(name, func(t *testing.T) { assertRefusedAsUnanswered(t, git, scan) })
	}
}

// A directory that git says is no repository is left to the scan, whose own read says so.
func TestRefuseForeignGitDirectoryLeavesADirectoryThatIsNoRepositoryToTheScan(t *testing.T) {
	t.Setenv("GIT_WORK_TREE", "")
	scan := t.TempDir()
	_, said := RunBytes(scan, "rev-parse", "--is-inside-work-tree")
	cases := map[string]error{
		"as git said it":             said,
		"with the message it prints": gitWaitError(exitStatus128, []byte("fatal: not a git repository (or any of the parent directories): .git\n")),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			git := answers{insideQuestion: {err: err}}

			if got := RefuseForeignGitDirectory(git.run, scan); got != nil {
				t.Errorf("RefuseForeignGitDirectory = %v; want nil, for a directory git says is no repository", got)
			}
		})
	}
}

func TestRefuseForeignGitDirectoryNamesTheGitDirectoryOfAForeignRepository(t *testing.T) {
	t.Setenv("GIT_WORK_TREE", "")
	root := t.TempDir()
	scan, gitDir := filepath.Join(root, "scan"), filepath.Join(root, "other", ".git")
	makeDirectories(t, scan, gitDir)
	git := answers{
		insideQuestion:  {output: "true\n"},
		layoutQuestion:  {output: gitDir + "\n" + gitDir + "\n"},
		worktreeSetting: {err: exitStatusOne},
	}

	err := RefuseForeignGitDirectory(git.run, scan)

	var foreign *ForeignGitDirectoryError
	if !errors.As(err, &foreign) || foreign.GitDir != gitDir {
		t.Fatalf("RefuseForeignGitDirectory = %v; want a refusal naming %s", err, gitDir)
	}
}
