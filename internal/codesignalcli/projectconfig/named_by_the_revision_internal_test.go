package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// What the analyzed revision commits at a path does not depend on the disk, so a policy committed as a link, a
// directory or a submodule has the word for that whether or not something is on disk. The disk's own words are for
// what the revision does not say: an entry on disk that the revision does not commit, and a parent that is a link.
func TestAPathTheRevisionCommitsAsANonFileHasThatWordWhateverIsOnDisk(t *testing.T) {
	cases := map[string]struct {
		prepare func(t *testing.T, repo string)
		path    string
		want    projectreadiness.PolicyCause
	}{
		"a directory committed and on disk":                              {commitDirectory, "cfg", projectreadiness.CauseDirectory},
		"a link committed and on disk as a link":                         {commitLinkAndKeepIt, "cfg/p.json", projectreadiness.CauseCommittedAsLink},
		"a submodule committed, its checkout there":                      {commitSubmoduleWithCheckout, "vendor", projectreadiness.CauseInSubmodule},
		"a parent the process cannot search, over a committed directory": {commitDirectoryBehindAParentThatCannotBeSearched, "cfg/sub", projectreadiness.CauseUnreadableOnDisk},
		"a directory on disk that nothing commits":                       {makeDirectory, "dir.json", projectreadiness.CauseNotARegularFile},
		"a link on disk that nothing commits":                            {makeLink, "link.json", projectreadiness.CauseSymbolicLink},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireCauseOnDisk(t, c.prepare, c.path, c.want) })
	}
}

func requireCauseOnDisk(t *testing.T, prepare func(*testing.T, string), path string, want projectreadiness.PolicyCause) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	prepare(t, repo)

	status, err := ExplicitOnDiskStatus(NewGitRunner(), repo, "HEAD", path)

	if err != nil || status == nil || status.PolicyCause != want {
		t.Errorf("ExplicitOnDiskStatus(%q) = %+v, %v; want the cause %q", path, status, err, want)
	}
}

func commitDirectory(t *testing.T, repo string) {
	t.Helper()
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
}

func commitLinkAndKeepIt(t *testing.T, repo string) {
	t.Helper()
	gitfixture.CommitLink(t, repo, "cfg/p.json", "../real.json")
	linkAt(t, repo, "cfg/p.json")
}

func commitSubmoduleWithCheckout(t *testing.T, repo string) {
	t.Helper()
	gitfixture.CommitGitlink(t, repo, "vendor")
	if err := os.MkdirAll(filepath.Join(repo, "vendor"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func makeDirectory(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, "dir.json"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func makeLink(t *testing.T, repo string) {
	t.Helper()
	linkAt(t, repo, "link.json")
}

func linkAt(t *testing.T, repo, path string) {
	t.Helper()
	full := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink("../real.json", full); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

func commitDirectoryBehindAParentThatCannotBeSearched(t *testing.T, repo string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root can search any directory")
	}
	gitfixture.CommitFile(t, repo, "cfg/sub/x.json", "{}")
	parent := filepath.Join(repo, "cfg")
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
}

// A question git cannot answer about what the revision commits is an operational failure, not a verdict: the
// refusal of the disk is not reported in its place. All of git is the fake: it says where the repository is, fails
// the tree question and fails the test on any other, so the failure returned is the one the question under test
// made.
func TestAFailedQuestionAboutWhatTheRevisionCommitsIsNotAVerdictAboutThePath(t *testing.T) {
	repo := t.TempDir()
	makeDirectory(t, repo)
	var unexpected []string
	allOfGit := func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "rev-parse" && slices.Contains(args, "--show-toplevel"):
			return []byte(repo + "\n"), nil
		case args[0] == "ls-tree":
			return nil, errBoom
		}
		unexpected = append(unexpected, strings.Join(args, " "))
		return nil, errNotAsked
	}

	status, err := ExplicitOnDiskStatus(allOfGit, repo, "HEAD", "dir.json")

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || !errors.Is(operational.Cause, errBoom) {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want the operational error of the failed tree question", status, err)
	}
	if len(unexpected) != 0 {
		t.Errorf("git was asked %q before the tree question failed; the fake answers only that one", unexpected)
	}
}

var errNotAsked = errors.New("git was asked a question the fake does not answer")
