package gitrepo

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestDirectoryEntriesAtRevisionListsTheNamesDirectlyUnderADirectory(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	gitfixture.CommitFile(t, repo, "cfg/sub/x.json", "{}")
	gitfixture.CommitFile(t, repo, "top.json", "{}")
	gitfixture.CommitFile(t, repo, "we ird/it's é.json", "{}")
	cases := []struct {
		name      string
		directory string
		want      []string
	}{
		{"the root", "", []string{"cfg", "top.json", "we ird"}},
		{"a directory, files and directories", "cfg", []string{"p.json", "sub"}},
		{"a nested directory", "cfg/sub", []string{"x.json"}},
		{"names a shell or a terminal would treat specially", "we ird", []string{"it's é.json"}},
		{"a directory in another case", "CFG", nil},
		{"a directory the tree does not hold", "absent", nil},
		{"a file, which has no entries", "cfg/p.json", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertDirectoryEntries(t, repo, c.directory, c.want)
		})
	}
}

func assertDirectoryEntries(t *testing.T, repo, directory string, want []string) {
	t.Helper()
	got, err := DirectoryEntriesAtRevision(RunBytes, repo, "HEAD", directory)

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("DirectoryEntriesAtRevision(%q) = %q, %v; want %q", directory, got, err, want)
	}
}

func TestDirectoryEntriesAtRevisionAsksForTheLiteralDirectoryFromTheRepositoryRoot(t *testing.T) {
	var gotArgs []string
	run := func(_ string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	if _, err := DirectoryEntriesAtRevision(run, "repo", "abc123", "a[1]*"); err != nil {
		t.Fatalf("DirectoryEntriesAtRevision: %v", err)
	}

	want := []string{"ls-tree", "--full-tree", "--name-only", "-z", "abc123", "--", ":(top,literal)a[1]*/"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("git args = %v; want %v", gotArgs, want)
	}
}

func TestDirectoryEntriesAtRevisionReportsGitsFailureAsAnErrorNotAsAnEmptyDirectory(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }

	got, err := DirectoryEntriesAtRevision(run, "repo", "HEAD", "cfg")

	if err == nil || got != nil {
		t.Errorf("DirectoryEntriesAtRevision = %q, %v; want an error and no names", got, err)
	}
}
