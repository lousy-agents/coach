package projectconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestJudgeRestore(t *testing.T) {
	const policy = "pol.json"
	cases := []struct {
		name  string
		setup func(t *testing.T, repo string)
		want  restoreVerdict
	}{
		{"a policy that was never committed", func(t *testing.T, repo string) {}, restoreNotCommitted},
		{"a committed policy with a clean index", func(t *testing.T, repo string) { commitPolicy(t, repo, policy, "{}", 0o644) }, restoreOverwritesNothing},
		{"a committed executable policy", func(t *testing.T, repo string) { commitPolicy(t, repo, policy, "{}", 0o755) }, restoreOverwritesNothing},
		{"a committed policy whose deletion was staged", func(t *testing.T, repo string) {
			commitPolicy(t, repo, policy, "{}", 0o644)
			runGit(t, repo, "rm", "-q", policy)
		}, restoreOverwritesNothing},
		{"a committed policy with a staged edit", func(t *testing.T, repo string) {
			commitPolicy(t, repo, policy, "{}", 0o644)
			writeFile(t, repo, policy, `{"edited":true}`)
			runGit(t, repo, "add", policy)
		}, restoreDiscardsStagedEdit},
		{"a committed policy with an unstaged edit", func(t *testing.T, repo string) {
			commitPolicy(t, repo, policy, "{}", 0o644)
			writeFile(t, repo, policy, `{"edited":true}`)
		}, restoreOverwritesNothing},
		{"a policy committed as a symbolic link", func(t *testing.T, repo string) {
			if err := os.Symlink("real.json", filepath.Join(repo, policy)); err != nil {
				t.Fatalf("symlink: %v", err)
			}
			runGit(t, repo, "add", policy)
			commitStaged(t, repo)
		}, restoreRecreatesLink},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertJudgeRestore(t, policy, c.setup, c.want)
		})
	}
}

func assertJudgeRestore(t *testing.T, policy string, setup func(t *testing.T, repo string), want restoreVerdict) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.go", "package a\n")
	setup(t, repo)

	if got := judgeRestore(gitrepo.RunBytes, repo, "HEAD", policy).verdict; got != want {
		t.Errorf("judgeRestore = %d; want %d", got, want)
	}
}

func TestJudgeRestoreDeclinesWhenTheRevisionCannotBeResolved(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.go", "package a\n")

	if got := judgeRestore(gitrepo.RunBytes, repo, "no-such-revision", "pol.json").verdict; got != restoreUndecidable {
		t.Errorf("judgeRestore = %d; want restoreUndecidable (%d)", got, restoreUndecidable)
	}
}

func commitPolicy(t *testing.T, repo, name, contents string, mode os.FileMode) {
	t.Helper()
	writeFile(t, repo, name, contents)
	if err := os.Chmod(filepath.Join(repo, name), mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	runGit(t, repo, "add", name)
	commitStaged(t, repo)
}

func writeFile(t *testing.T, repo, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func commitStaged(t *testing.T, repo string) {
	t.Helper()
	command := exec.Command("git", "commit", "-q", "-m", "commit")
	command.Dir = repo
	command.Env = gitfixture.CommitEnv
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
