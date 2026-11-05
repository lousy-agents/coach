package gitrepo

import "strings"

// The methods below are where the scan's file selection, its counts and its
// disclosure read a porcelain XY code as a meaning about a path, so a case git
// adds to the vocabulary is decided once. ClassifyWorktreeStatus groups a code
// for the committed scan's summary.

// IsUntracked is true for a path git lists as untracked.
func (e WorktreeEntry) IsUntracked() bool {
	return e.Code == "??"
}

// IsUnmerged is true for a path a conflicted merge left unresolved, in any of
// the seven states git reports. HEAD holds a version of it and so does the disk.
func (e WorktreeEntry) IsUnmerged() bool {
	return worktreeStatusUnmerged(e.Code)
}

// IsNew is true for a path the working tree holds and HEAD does not: an
// untracked file, a staged or intent-to-add addition, and the new path of a
// rename or copy. A new path the working tree then deleted is none of these:
// nothing is left to read. An unmerged path is not new even when git reports an
// addition (AA, AU): HEAD holds it.
func (e WorktreeEntry) IsNew() bool {
	return !e.IsUnmerged() && (e.IsUntracked() || AddsPath(e.Code))
}

// IsEditedTracked is true for a tracked file whose content or type changed and
// which is not added, renamed, deleted or unmerged.
func (e WorktreeEntry) IsEditedTracked() bool {
	if len(e.Code) < 2 || e.IsUntracked() || e.Code == "!!" || e.Code == "  " {
		return false
	}
	return editedColumn(e.Code[0]) && editedColumn(e.Code[1])
}

func editedColumn(c byte) bool {
	return c == ' ' || c == 'M' || c == 'T'
}

// IsDeletedFromDisk is true for a tracked file that is gone from disk, staged or
// not. A file added to the index and then deleted (AD, RD, CD) was never in
// HEAD, and an unmerged state has no single answer.
func (e WorktreeEntry) IsDeletedFromDisk() bool {
	switch e.Code {
	case " D", "D ", "MD", "TD":
		return true
	}
	return false
}

// RenameSource returns the path a rename moved this entry from, which HEAD
// holds and the move took off disk. A copy keeps its source, so it has none.
func (e WorktreeEntry) RenameSource() (string, bool) {
	return e.OldPath, e.OldPath != "" && strings.Contains(e.Code, "R")
}

// RenameSourceGone is true for the source of a rename whose new path the working
// tree then deleted (RD): HEAD holds the source, and neither it nor its
// destination is on disk. A copy leaves its source in place.
func (e WorktreeEntry) RenameSourceGone() bool {
	return e.OldPath != "" && e.Code == "RD"
}
