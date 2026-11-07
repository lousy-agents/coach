package gitrepo

import (
	"fmt"
	"os"
)

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
