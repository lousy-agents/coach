package gitrepo

import (
	"os"
)

// writeSnapshot writes the entries below dir. Each blob is read once and written to every path that names
// it. A submodule is a commit of another repository, so it becomes an empty directory. Every write goes
// through one os.Root, so a tree that names a path outside dir, or through a link, is an error and not a
// file written elsewhere.
//
// A blob past the limit on one object is not held, and a blob that would pass the total the snapshot may
// write is not written; either one is a zero-length file at each of its paths, so the tree keeps its shape
// and a //go:embed pattern still finds the file. The files the scan reads are written first, so the total
// is spent on them: it fails the snapshot only when those alone pass it.
func writeSnapshot(repositoryRoot, dir string, entries []snapshotEntry) error {
	return writeSnapshotWithin(repositoryRoot, dir, entries, snapshotTotal)
}

// writeSnapshotWithin is writeSnapshot with a limit on what it writes in all, counting a blob once for every
// path that names it.
func writeSnapshotWithin(repositoryRoot, dir string, entries []snapshotEntry, total int64) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	blobs, err := makeSubmoduleDirectories(root, entries)
	if err != nil {
		return err
	}
	order, byObject := groupByObject(neededFirst(blobs))
	writer := &snapshotWriter{root: root, byObject: byObject, total: total}
	outcome, err := readObjects(repositoryRoot, order, defaultSnapshotBudget, writer.write)
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
