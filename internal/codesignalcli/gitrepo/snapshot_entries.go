package gitrepo

import (
	"fmt"
	"strings"
)

const (
	modeExecutable = "100755"
	modeSymlink    = "120000"
	modeGitlink    = "160000"
)

// snapshotEntry is one path of a tree: its mode, the object it names, and the path.
type snapshotEntry struct{ mode, oid, path string }

// parseSnapshotEntries reads `git ls-tree -r -z`: "<mode> <type> <object>\t<path>" ending in NUL. A tree
// that holds a path git would not check out is refused whole: `git archive` refused it, and skipping the entry
// would scan a tree other than the one committed and report on it as if it were whole.
func parseSnapshotEntries(listing []byte) ([]snapshotEntry, error) {
	var entries []snapshotEntry
	for _, record := range strings.Split(string(listing), "\x00") {
		if record == "" {
			continue
		}
		meta, path, found := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !found || len(fields) != 3 || path == "" {
			return nil, fmt.Errorf("unexpected ls-tree entry %q", record)
		}
		if err := refuseUncheckablePath(path); err != nil {
			return nil, err
		}
		entries = append(entries, snapshotEntry{mode: fields[0], oid: fields[2], path: path})
	}
	return entries, nil
}
