package gitrepo

import (
	"fmt"
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
	if len(outcome.missing) > 0 {
		return &ObjectsNotLocalError{Count: len(outcome.missing), FirstPath: byObject[outcome.missing[0]][0].path}
	}
	return nil
}

// snapshotWriter writes each object it is handed, within the total.
type snapshotWriter struct {
	root     *os.Root
	byObject map[string][]snapshotEntry
	total    int64
	written  int64
}

func (w *snapshotWriter) write(oid string, content []byte) error {
	entries := w.byObject[oid]
	size := int64(len(content)) * int64(len(entries))
	if w.written+size <= w.total {
		w.written += size
		return writeEveryPathOf(w.root, entries, content)
	}
	if anyNeedsContent(entries) {
		return fmt.Errorf("the files the scan reads from the source snapshot are larger than %d bytes", w.total)
	}
	return writePlaceholders(w.root, entries)
}

func writeEveryPathOf(root *os.Root, entries []snapshotEntry, content []byte) error {
	for _, entry := range entries {
		if err := writeSnapshotEntry(root, entry, content); err != nil {
			return err
		}
	}
	return nil
}

// writePlaceholders writes a zero-length file at each path of a blob whose bytes are dropped. A symbolic
// link has no such form (its content is its target), and a link is never large, so it is left out.
func writePlaceholders(root *os.Root, entries []snapshotEntry) error {
	for _, entry := range entries {
		if entry.mode == modeSymlink {
			continue
		}
		if err := writeSnapshotEntry(root, entry, nil); err != nil {
			return err
		}
	}
	return nil
}
