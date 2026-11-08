package projectconfig

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

const replacePath = "cfg/p.json"

// hashed writes content as a blob and returns its id.
func hashed(t *testing.T, repo, content string) string {
	t.Helper()
	command := exec.Command("git", "-C", repo, "hash-object", "-w", "--stdin")
	command.Stdin = strings.NewReader(content)
	out, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// recordAConflict leaves the index with the base and both sides of path, and no entry at stage 0, as a merge that
// stopped on it does.
func recordAConflict(t *testing.T, repo, path string) {
	t.Helper()
	if err := exec.Command("git", "-C", repo, "rm", "-q", "--cached", "--", path).Run(); err != nil {
		t.Fatal(err)
	}
	entries := ""
	for stage, content := range map[string]string{"1": "base\n", "2": "ours\n", "3": "theirs\n"} {
		entries += "100644 " + hashed(t, repo, content) + " " + stage + "\t" + path + "\n"
	}
	command := exec.Command("git", "-C", repo, "update-index", "--index-info")
	command.Stdin = strings.NewReader(entries)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("update-index: %v\n%s", err, out)
	}
}

func committedInvalidPolicy(t *testing.T) string {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, replacePath, `{"bogus":1}`)
	return repo
}

// The state of a committed policy that cannot be used decides whether a new document may replace it: only a file that
// is the committed copy, or is gone, with nothing staged or in conflict, has no work in it.
func TestTheWorkingCopyOfACommittedPolicyIsToldFromTheIndexAndTheDisk(t *testing.T) {
	cases := map[string]struct {
		arrange func(t *testing.T, repo string)
		want    workingCopy
	}{
		"the committed copy on disk": {func(*testing.T, string) {}, workingCopyUntouched},
		"gone from disk":             {func(t *testing.T, repo string) { moveAside(t, repo, replacePath) }, workingCopyUntouched},
		"edited on disk": {func(t *testing.T, repo string) {
			writeFile(t, repo, replacePath, validPolicy)
		}, workingCopyChanged},
		"edited and staged": {func(t *testing.T, repo string) {
			writeFile(t, repo, replacePath, validPolicy)
			stage(t, repo, replacePath)
		}, workingCopyStaged},
		"edited, staged and edited again": {func(t *testing.T, repo string) {
			writeFile(t, repo, replacePath, validPolicy)
			stage(t, repo, replacePath)
			writeFile(t, repo, replacePath, `{"schema_version":"1","roots":["a"]}`)
		}, workingCopyStaged},
		"edited on disk and removed from the index, so untracked": {func(t *testing.T, repo string) {
			writeFile(t, repo, replacePath, validPolicy)
			if err := exec.Command("git", "-C", repo, "rm", "-q", "--cached", "--", replacePath).Run(); err != nil {
				t.Fatal(err)
			}
		}, workingCopyChanged},
		"edited under a skip-worktree mark, which git does not look at": {func(t *testing.T, repo string) {
			gitfixture.Git(t, repo, "update-index", "--skip-worktree", replacePath)
			writeFile(t, repo, replacePath, validPolicy)
		}, workingCopyChanged},
		"edited under an assume-unchanged mark, which git does not look at": {func(t *testing.T, repo string) {
			gitfixture.Git(t, repo, "update-index", "--assume-unchanged", replacePath)
			writeFile(t, repo, replacePath, validPolicy)
		}, workingCopyChanged},
		"touched, so its stat is dirty, with the committed bytes": {func(t *testing.T, repo string) {
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(repo, replacePath), later, later); err != nil {
				t.Fatal(err)
			}
		}, workingCopyUntouched},
		"in conflict": {func(t *testing.T, repo string) { recordAConflict(t, repo, replacePath) }, workingCopyConflicted},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheWorkingCopy(t, c.arrange, c.want) })
	}
}

func requireTheWorkingCopy(t *testing.T, arrange func(t *testing.T, repo string), want workingCopy) {
	t.Helper()
	repo := committedInvalidPolicy(t)
	arrange(t, repo)

	got, err := inspectWorkingCopy(NewGitRunner(), repo, "HEAD", replacePath)

	if err != nil || got != want {
		t.Errorf("inspectWorkingCopy = %d, %v; want %d", got, err, want)
	}
}

// asConfigError is whether err is a *ConfigError, which it is stored into.
func asConfigError(err error, target **ConfigError) bool {
	return errors.As(err, target)
}
