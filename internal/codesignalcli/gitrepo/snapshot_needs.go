package gitrepo

import (
	"path/filepath"
	"strings"
)

// neededExtensions are the kinds of file the scan reads from the source snapshot: the sources of the
// languages it analyzes, the Go module files, and the JSON a TypeScript project is configured by. The rest of
// a tree (images, archives, data) is not read by any analyzer, but go list needs its paths to exist, since a
// //go:embed pattern that matches nothing fails it; so the rest is written when the budget allows, and as
// a zero-length file when it does not.
var neededExtensions = map[string]bool{
	".go": true, ".mod": true, ".sum": true, ".work": true,
	".ts": true, ".tsx": true, ".mts": true, ".cts": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".json": true, ".jsonc": true,
}

func needsContent(path string) bool {
	return neededExtensions[strings.ToLower(filepath.Ext(path))]
}

// neededFirst lists the entries a scan reads before the others, each group in its listing order, so that a
// snapshot that has to stop writing stops with the files nothing reads.
func neededFirst(entries []snapshotEntry) []snapshotEntry {
	ordered := make([]snapshotEntry, 0, len(entries))
	for _, entry := range entries {
		if needsContent(entry.path) {
			ordered = append(ordered, entry)
		}
	}
	for _, entry := range entries {
		if !needsContent(entry.path) {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

func anyNeedsContent(entries []snapshotEntry) bool {
	for _, entry := range entries {
		if needsContent(entry.path) {
			return true
		}
	}
	return false
}
