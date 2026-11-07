package gitrepo

import (
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The kind comes from what git records, so a link, a directory and a submodule entry are told apart
// from a file by their mode and type, in a repository git built, from any directory in it.
func TestEntryKindAtRevisionTellsWhatARealRepositoryRecords(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/file.json", "{}")
	gitfixture.CommitFile(t, repo, "cfg/dir.json/inner.json", "{}")
	gitfixture.CommitLink(t, repo, "cfg/link.json", "file.json")
	gitfixture.CommitGitlink(t, repo, "cfg/sub.json")
	cases := map[string]struct {
		path string
		want EntryKind
	}{
		"a regular file":    {"cfg/file.json", EntryRegularFile},
		"a symbolic link":   {"cfg/link.json", EntrySymbolicLink},
		"a directory":       {"cfg/dir.json", EntryDirectory},
		"a submodule entry": {"cfg/sub.json", EntrySubmodule},
		"nothing":           {"cfg/none.json", EntryAbsent},
		"a name that is a prefix of one that exists": {"cfg/file", EntryAbsent},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheKindFromEveryDirectory(t, repo, c.path, c.want) })
	}
}

func requireTheKindFromEveryDirectory(t *testing.T, repo, path string, want EntryKind) {
	t.Helper()
	for _, dir := range []string{"", "cfg", "cfg/dir.json"} {
		got, err := EntryKindAtRevision(RunBytes, filepath.Join(repo, dir), "HEAD", path)
		if err != nil || got != want {
			t.Errorf("EntryKindAtRevision from %q for %q = %v, %v; want %v", dir, path, got, err, want)
		}
	}
}

// A path that is spelled like pathspec magic is the name of a file, as it is everywhere else the
// policy is read: ":/cfg.json" is not cfg.json from the root.
func TestEntryKindAtRevisionTakesThePathLiterally(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg.json", "{}")
	got, err := EntryKindAtRevision(RunBytes, repo, "HEAD", ":/cfg.json")
	if err != nil || got != EntryAbsent {
		t.Errorf("EntryKindAtRevision for a name spelled as pathspec magic = %v, %v; want %v: the path is a name, not a pathspec", got, err, EntryAbsent)
	}
}
