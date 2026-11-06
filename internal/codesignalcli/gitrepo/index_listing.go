package gitrepo

import (
	"strconv"
	"strings"
)

// parseIndexListing reads ls-files --stage -t --debug output: an entry line, then
// the debug lines that describe it, for each entry.
func parseIndexListing(listing string) ([]IndexEntry, error) {
	var entries []IndexEntry
	for _, group := range groupEntryLines(strings.Split(listing, "\n")) {
		entry, err := parseIndexEntry(group)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// groupEntryLines gathers each entry line with the indented debug lines under it.
// An indented line with nothing above it starts a group of its own, which does not
// parse as an entry.
func groupEntryLines(lines []string) [][]string {
	var groups [][]string
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") && len(groups) > 0 {
			groups[len(groups)-1] = append(groups[len(groups)-1], line)
			continue
		}
		groups = append(groups, []string{line})
	}
	return groups
}

// parseIndexEntry reads one entry line and the debug lines under it.
func parseIndexEntry(group []string) (IndexEntry, error) {
	meta, _, found := strings.Cut(group[0], "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 4 {
		return IndexEntry{}, errUnparseableIndexLine
	}
	intentToAdd, err := intentToAddIn(group[1:])
	if err != nil {
		return IndexEntry{}, err
	}
	return IndexEntry{TreeEntry: TreeEntry{Mode: fields[1], SHA: fields[2]}, Stage: fields[3], SkipWorktree: fields[0] == skipWorktreeTag, IntentToAdd: intentToAdd}, nil
}

// intentToAddIn reads the flags line among an entry's debug lines. An entry with
// no flags line is not intent-to-add, and flags it cannot read are an error.
func intentToAddIn(debugLines []string) (bool, error) {
	for _, line := range debugLines {
		_, rest, hasFlags := strings.Cut(line, "flags: ")
		if !hasFlags {
			continue
		}
		flags, err := strconv.ParseUint(strings.TrimSpace(rest), 16, 32)
		if err != nil {
			return false, errUnparseableIndexLine
		}
		return flags&intentToAddFlag != 0, nil
	}
	return false, nil
}
