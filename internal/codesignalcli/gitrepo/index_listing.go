package gitrepo

import "strings"

// parseIndexListing reads ls-files --stage -t output, one entry per line.
func parseIndexListing(listing string) ([]IndexEntry, error) {
	var entries []IndexEntry
	for _, line := range strings.Split(listing, "\n") {
		entry, err := parseIndexLine(line)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseIndexLine reads "<tag> <mode> <object id> <stage>\t<path>". Every field is
// checked, because an entry is judged by what it holds and a field read wrongly
// would judge it as something it is not.
func parseIndexLine(line string) (IndexEntry, error) {
	meta, _, found := strings.Cut(line, "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 4 || len(fields[0]) != 1 || !isOctalMode(fields[1]) || !isObjectID(fields[2]) || !isStage(fields[3]) {
		return IndexEntry{}, errUnparseableIndexLine
	}
	return IndexEntry{TreeEntry: TreeEntry{Mode: fields[1], SHA: fields[2]}, Stage: fields[3], SkipWorktree: fields[0] == skipWorktreeTag}, nil
}

// isOctalMode is true for the six octal digits of a git file mode.
func isOctalMode(mode string) bool {
	return len(mode) == 6 && strings.Trim(mode, "01234567") == ""
}

// isObjectID is true for a SHA-1 or SHA-256 object id in lowercase hexadecimal.
func isObjectID(id string) bool {
	return (len(id) == 40 || len(id) == 64) && strings.Trim(id, "0123456789abcdef") == ""
}

// isStage is true for the stages of an index entry: 0, or 1 to 3 in a conflict.
func isStage(stage string) bool {
	return len(stage) == 1 && strings.Contains("0123", stage)
}
