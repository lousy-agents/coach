package gitrepo

import (
	"io"
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
	entries, err := listSnapshotEntries(repository, "HEAD")
	if err != nil {
		t.Fatalf("list the tree: %v", err)
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

// snapshotWriteHeldTo is the write of a real snapshot held to object bytes for one object and total bytes in all.
func snapshotWriteHeldTo(object, total int64) snapshotWrite {
	write := defaultSnapshotWrite()
	write.budget.object, write.total = object, total
	return write
}

// creatingFiles is write with every file of the snapshot being file, whose write or close may fail as a full disk's do.
func creatingFiles(write snapshotWrite, file io.WriteCloser) snapshotWrite {
	write.create = creating(file)
	return write
}

// readingThrough is write with its objects read through the git that build starts.
func readingThrough(write snapshotWrite, build childBuilder) snapshotWrite {
	write.child = build
	return write
}

// creating is the file creator whose every file is file.
func creating(file io.WriteCloser) fileCreator {
	return func(*os.Root, string, os.FileMode) (io.WriteCloser, error) { return file, nil }
}

// directoryCreating is a directory of a snapshot below root whose every file is file.
func directoryCreating(root *os.Root, file io.WriteCloser) snapshotDir {
	return snapshotDir{Root: root, create: creating(file)}
}

// realDirectory is a directory of a real snapshot below root.
func realDirectory(root *os.Root) snapshotDir {
	return snapshotDir{Root: root, create: createSnapshotFile}
}

// extractedWith is ExtractRevision of the repository's HEAD under write.
func extractedWith(write snapshotWrite, repository string) (string, error) {
	return extractRevision(write, listSnapshotEntries, RunBytes, repository, "HEAD")
}
