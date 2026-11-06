package gitrepo

import "strings"

// lsTreeRecord is the one entry `git ls-tree` prints for a path: its mode, its object type and its object name.
type lsTreeRecord struct {
	mode       string
	objectType string
	object     string
}

// readLsTreeRecord reads the output of `git ls-tree -z --full-tree <revision> -- :(top,literal)<repoPath>`, the
// one question every caller of this package asks about what a revision holds at a path, so that one parser
// answers it. No output is no record. A record for any other path, a second record, or one that is not
// "<mode> <type> <object>\t<path>" is an error: guessing would call a directory a file or a file absent, and
// a project git printed something unreadable for is not a project without that file.
func readLsTreeRecord(output []byte, repoPath string) (record lsTreeRecord, found bool, err error) {
	records := SplitNULPaths(output)
	if len(records) == 0 {
		return lsTreeRecord{}, false, nil
	}
	record, err = parseLsTreeRecord(records[0], repoPath)
	if err != nil || len(records) > 1 {
		return lsTreeRecord{}, false, errUnparseableIndexLine
	}
	return record, true, nil
}

// parseLsTreeRecord reads "<mode> <type> <object>\t<path>" and refuses a record for another path.
func parseLsTreeRecord(record, repoPath string) (lsTreeRecord, error) {
	meta, path, found := strings.Cut(record, "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 3 || !SamePath(path, repoPath) {
		return lsTreeRecord{}, errUnparseableIndexLine
	}
	return lsTreeRecord{mode: fields[0], objectType: fields[1], object: fields[2]}, nil
}
