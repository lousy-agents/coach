package gitrepo

import (
	"io"
)

// writeLarge takes an object past the limit on one object when a path that names it is a file the scan reads:
// go list parses a source of that size, and an empty file in its place fails it. The object is written to disk
// a chunk at a time, so it is never held, and it counts against the total for every path that names it. An
// object that no path needs is left, to be read past and written as a placeholder.
func (w *snapshotWriter) writeLarge(oid string, size int64, body io.Reader) (taken bool, err error) {
	entries := w.byObject[oid]
	if !anyNeedsContent(entries) {
		return false, nil
	}
	if !w.spend(size * int64(len(entries))) {
		return false, w.pastTheTotal()
	}
	return true, streamEveryPathOf(w.root, entries, size, body)
}

// streamEveryPathOf writes the object to the first of its paths from the stream, and copies that file to the
// others, so that the stream is read once.
func streamEveryPathOf(root snapshotDir, entries []snapshotEntry, size int64, body io.Reader) error {
	files := regularFiles(entries)
	if len(files) == 0 {
		return nil
	}
	if err := streamSnapshotEntry(root, files[0], size, body); err != nil {
		return err
	}
	for _, entry := range files[1:] {
		if err := copySnapshotEntry(root, entry, files[0], size); err != nil {
			return err
		}
	}
	return nil
}

// regularFiles are the entries that are files. A symbolic link has no stream form (its content is its target),
// and a link is never large.
func regularFiles(entries []snapshotEntry) []snapshotEntry {
	var files []snapshotEntry
	for _, entry := range entries {
		if entry.mode != modeSymlink {
			files = append(files, entry)
		}
	}
	return files
}
