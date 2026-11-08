package gitrepo

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/gitpath"
)

const (
	modeExecutable = "100755"
	modeSymlink    = "120000"
	modeGitlink    = "160000"
)

// snapshotEntry is one path of a tree: its mode, the object it names, and the path.
type snapshotEntry struct{ mode, oid, path string }

// listSnapshotEntries reads `git ls-tree -r -z`: "<mode> <type> <object>\t<path>" ending in NUL, a record at a
// time. A tree that holds a path git would not check out is refused whole: `git archive` refused it, and skipping
// the entry would scan a tree other than the one committed and report on it as if it were whole.
func listSnapshotEntries(repositoryRoot, revision string) ([]snapshotEntry, error) {
	return listSnapshotEntriesBy(ReadCommandContext, repositoryRoot, revision)
}

// listSnapshotEntriesBy is listSnapshotEntries with the git child built by the caller.
func listSnapshotEntriesBy(build childBuilder, repositoryRoot, revision string) ([]snapshotEntry, error) {
	var entries []snapshotEntry
	err := streamListingBy(build, repositoryRoot, snapshotListingWallTime, []string{"ls-tree", "-r", "-z", "--full-tree", revision}, func(record []byte) error {
		entry, err := parseSnapshotRecord(string(record))
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func parseSnapshotRecord(record string) (snapshotEntry, error) {
	meta, path, found := strings.Cut(record, "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 3 || path == "" {
		return snapshotEntry{}, fmt.Errorf("unexpected ls-tree entry %q", record)
	}
	if err := gitpath.RefuseUncheckable(path); err != nil {
		return snapshotEntry{}, err
	}
	return snapshotEntry{mode: fields[0], oid: fields[2], path: path}, nil
}
