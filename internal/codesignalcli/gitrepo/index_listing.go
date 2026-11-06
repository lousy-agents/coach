package gitrepo

import "strings"

// parseIndexListing reads ls-files --stage -t -z output, one NUL-terminated record per
// entry (an empty one is refused, not skipped). Every entry must be for repoPath itself.
func parseIndexListing(listing []byte, repoPath string) ([]IndexEntry, error) {
	var entries []IndexEntry
	for _, record := range SplitNULPaths(listing) {
		entry, path, err := parseIndexRecord(record)
		if err != nil {
			return nil, err
		}
		if !SamePath(path, repoPath) {
			return nil, errIndexEntryForAnotherPath
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseIndexRecord reads "<tag> <mode> <object id> <stage>\t<path>". Every field is
// checked, because an entry is judged by what it holds and a field read wrongly
// would judge it as something it is not.
func parseIndexRecord(record string) (IndexEntry, string, error) {
	meta, path, found := strings.Cut(record, "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 4 || !isIndexTag(fields[0]) || !isOctalMode(fields[1]) || !isObjectID(fields[2]) || !isStage(fields[3]) {
		return IndexEntry{}, "", errUnparseableIndexLine
	}
	return IndexEntry{TreeEntry: TreeEntry{Mode: fields[1], SHA: fields[2]}, Stage: fields[3], SkipWorktree: fields[0] == skipWorktreeTag}, path, nil
}

// isIndexTag is true for the tags ls-files -t prints for an entry of the index:
// H for a cached file, S for a skip-worktree one and M for an unmerged one. The
// other tags git documents (R, C, K, ? and U) describe the listings that -d, -m,
// -o, -k and -u ask for, which coach does not request, so one in this listing is
// not something to read as a plain entry.
func isIndexTag(tag string) bool {
	return tag == "H" || tag == skipWorktreeTag || tag == "M"
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
