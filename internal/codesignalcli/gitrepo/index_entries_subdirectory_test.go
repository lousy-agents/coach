package gitrepo

import (
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// git prints index paths relative to the directory it runs in unless it is told to
// print them from the repository root, and a scan can start in a subdirectory. The
// path is compared with the one asked about, which is relative to the root.
func TestIndexEntriesFindsThePathFromASubdirectoryOfTheRepository(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	gitfixture.CommitFile(t, repo, "cfg/deeper/x.json", "{}")
	cases := map[string]struct {
		dir, path  string
		wantAtPath int
		wantBelow  bool
	}{
		"from the root":                     {"", "cfg/p.json", 1, false},
		"from the directory the file is in": {"cfg", "cfg/p.json", 1, false},
		"from a sibling directory":          {"cfg/deeper", "cfg/p.json", 1, false},
		"for a directory, from inside it":   {"cfg", "cfg", 0, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireIndexFrom(t, filepath.Join(repo, c.dir), c.path, c.wantAtPath, c.wantBelow) })
	}
}

func requireIndexFrom(t *testing.T, dir, path string, wantAtPath int, wantBelow bool) {
	t.Helper()
	got, err := IndexEntries(RunBytes, dir, path)
	if err != nil || len(got.Entries) != wantAtPath || got.HoldsBelow != wantBelow {
		t.Errorf("IndexEntries(%q, %q) = %+v, %v; want %d entries at the path and HoldsBelow %v", dir, path, got, err, wantAtPath, wantBelow)
	}
}
