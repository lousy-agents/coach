package gitrepo

import (
	"fmt"
)

// snapshotWriter writes each object it is handed, within the total.
type snapshotWriter struct {
	root     snapshotDir
	byObject map[string][]snapshotEntry
	total    int64
	written  int64
}

// spend counts size bytes against the total and reports whether they fit.
func (w *snapshotWriter) spend(size int64) bool {
	if w.written+size > w.total {
		return false
	}
	w.written += size
	return true
}

func (w *snapshotWriter) pastTheTotal() error {
	return fmt.Errorf("the files the scan reads from the source snapshot are larger than %d bytes", w.total)
}

func (w *snapshotWriter) write(oid string, content []byte) error {
	entries := w.byObject[oid]
	if w.spend(int64(len(content)) * int64(len(entries))) {
		return writeEveryPathOf(w.root, entries, content)
	}
	if anyNeedsContent(entries) {
		return w.pastTheTotal()
	}
	return writePlaceholders(w.root, entries)
}

func writeEveryPathOf(root snapshotDir, entries []snapshotEntry, content []byte) error {
	for _, entry := range entries {
		if err := writeSnapshotEntry(root, entry, content); err != nil {
			return err
		}
	}
	return nil
}

// writePlaceholders writes a zero-length file at each path of a blob whose bytes are dropped. A symbolic
// link has no such form (its content is its target), and a link is never large, so it is left out.
func writePlaceholders(root snapshotDir, entries []snapshotEntry) error {
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
