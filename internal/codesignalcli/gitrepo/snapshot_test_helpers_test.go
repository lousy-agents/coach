package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

type namedFile struct{ name, content string }

func snapshotOfFiles(t *testing.T, files ...namedFile) (repository string, entries []snapshotEntry) {
	t.Helper()
	repository = gitfixture.Init(t)
	for _, file := range files {
		gitfixture.CommitFile(t, repository, file.name, file.content)
	}
	listing, err := RunBytes(repository, "ls-tree", "-r", "-z", "--full-tree", "HEAD")
	if err != nil {
		t.Fatalf("ls-tree: %v", err)
	}
	entries, err = parseSnapshotEntries(listing)
	if err != nil {
		t.Fatalf("parse the listing: %v", err)
	}
	return repository, entries
}

func sizeOf(t *testing.T, dir, name string) int64 {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	return info.Size()
}

func withSnapshotLimits(t *testing.T, object, total int64) {
	t.Helper()
	originalBudget, originalTotal := defaultSnapshotBudget, snapshotTotal
	t.Cleanup(func() { defaultSnapshotBudget, snapshotTotal = originalBudget, originalTotal })
	defaultSnapshotBudget.object, snapshotTotal = object, total
}
