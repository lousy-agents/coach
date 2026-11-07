package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

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

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", path)

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
// refusal of the disk is not reported in its place.
func TestAFailedQuestionAboutWhatTheRevisionCommitsIsNotAVerdictAboutThePath(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	makeDirectory(t, repo)
	failing := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" {
			return nil, errBoom
		}
		return runProjectConfigGit(dir, args...)
	}

	status, err := ExplicitOnDiskStatus(failing, repo, "HEAD", "dir.json")

	if err == nil {
		t.Errorf("ExplicitOnDiskStatus = %+v, nil; want the failed question returned", status)
	}
}
