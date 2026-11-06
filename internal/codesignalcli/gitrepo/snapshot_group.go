package gitrepo

import (
	"os"
	"path/filepath"
)

// makeSubmoduleDirectories creates the directory for each submodule entry and returns the others.
func makeSubmoduleDirectories(root *os.Root, entries []snapshotEntry) ([]snapshotEntry, error) {
	var blobs []snapshotEntry
	for _, entry := range entries {
		if entry.mode != modeGitlink {
			blobs = append(blobs, entry)
			continue
		}
		if err := root.MkdirAll(filepath.FromSlash(entry.path), 0o755); err != nil {
			return nil, err
		}
	}
	return blobs, nil
}

// groupByObject lists the object ids in the order they first appear and the entries that name each.
func groupByObject(entries []snapshotEntry) (order []string, byObject map[string][]snapshotEntry) {
	byObject = map[string][]snapshotEntry{}
	for _, entry := range entries {
		if _, seen := byObject[entry.oid]; !seen {
			order = append(order, entry.oid)
		}
		byObject[entry.oid] = append(byObject[entry.oid], entry)
	}
	return order, byObject
}
