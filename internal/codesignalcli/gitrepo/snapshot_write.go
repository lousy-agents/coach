package gitrepo

import (
	"os"
)

// writeSnapshot writes the entries below dir. Each blob is read once and written to every path that names
// it. A submodule is a commit of another repository, so it becomes an empty directory. Every write goes
// through one os.Root, so a tree that names a path outside dir, or through a link, is an error and not a
// file written elsewhere.
//
// A blob past the limit on one object is not held. When a path that names it is a file the scan reads, it is
// written to disk a chunk at a time and in full, since go list parses it; otherwise it is a zero-length file
// at each of its paths, so the tree keeps its shape and a //go:embed pattern still finds the file. A blob that
// would pass the total the snapshot may write is not written either way: a zero-length file for a file no
// scan reads, and a refusal for one it does. The files the scan reads are written first, so the total is
// spent on them: it fails the snapshot only when those alone pass it.
func writeSnapshot(repositoryRoot, dir string, entries []snapshotEntry) error {
	return writeSnapshotWith(defaultSnapshotWrite(), repositoryRoot, dir, entries)
}

// snapshotWrite is what writing a snapshot is held to and done with besides the repository and the entries: the most it
// may write in all, counting a blob once for every path that names it, the budget and the child of the git that reads
// the objects, and how a file is created. A real snapshot gets defaultSnapshotWrite. A test hands in a small total or a small
// budget, or a creator whose file fails, so that no package variable is replaced.
type snapshotWrite struct {
	total  int64
	budget snapshotBudget
	child  childBuilder
	create fileCreator
}

func defaultSnapshotWrite() snapshotWrite {
	return snapshotWrite{total: snapshotTotalBytes, budget: defaultSnapshotBudget(), child: ReadCommandContext, create: createSnapshotFile}
}

// writeSnapshotWith is writeSnapshot held to the limits, and made with the file creator, of write.
func writeSnapshotWith(write snapshotWrite, repositoryRoot, dir string, entries []snapshotEntry) error {
	opened, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = opened.Close() }()
	root := snapshotDir{Root: opened, create: write.create}
	blobs, err := makeSubmoduleDirectories(opened, entries)
	if err != nil {
		return err
	}
	order, byObject := groupByObject(neededFirst(blobs))
	writer := &snapshotWriter{root: root, byObject: byObject, total: write.total}
	outcome, err := readObjects(write.child, repositoryRoot, order, write.budget, writer.write, writer.writeLarge)
	if err != nil {
		return err
	}
	for _, oid := range outcome.tooLarge {
		if err := writePlaceholders(root, byObject[oid]); err != nil {
			return err
		}
	}
	if len(outcome.unreadable) > 0 {
		return &ObjectsUnreadableError{Count: len(outcome.unreadable), FirstPath: byObject[outcome.unreadable[0]][0].path, Said: outcome.said}
	}
	if len(outcome.missing) > 0 {
		return &ObjectsNotLocalError{Count: len(outcome.missing), FirstPath: byObject[outcome.missing[0]][0].path}
	}
	return nil
}
