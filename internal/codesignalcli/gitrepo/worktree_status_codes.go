package gitrepo

import "strings"

func ClassifyWorktreeStatus(code string) (untracked, staged, modified, unmerged bool) {
	if len(code) < 2 || code == "!!" {
		return false, false, false, false
	}
	if code == "??" || worktreeIntentToAdd(code) {
		return true, false, false, false
	}
	if worktreeStatusUnmerged(code) {
		return false, false, false, true
	}
	staged = worktreeIndexStaged(code[0])
	modified = worktreeColumnModified(code[1])
	if !staged && !modified {
		modified = true
	}
	return false, staged, modified, false
}

// NewPathDeletedFromDisk is true for an added, renamed or copied path that the
// working tree then deleted (AD, RD, CD). HEAD never held that path and the disk
// no longer does, so there is nothing at it to read or to count.
func NewPathDeletedFromDisk(code string) bool {
	return len(code) == 2 && code[1] == 'D' && (code[0] == 'A' || code[0] == 'R' || code[0] == 'C')
}

// AddsPath is true for a record whose path is new in the working tree: an
// addition (staged or intent-to-add), and the destination of a rename or copy in
// whichever column git reports it. `git add -N` on a moved file reports the
// rename in the worktree column (" R old -> new"). A path the working tree then
// deleted is not new.
func AddsPath(code string) bool {
	if len(code) < 2 || NewPathDeletedFromDisk(code) {
		return false
	}
	return code == " A" || strings.ContainsRune("ARC", rune(code[0])) || strings.ContainsRune("RC", rune(code[1]))
}

func worktreeIntentToAdd(code string) bool {
	return code[0] == ' ' && code[1] == 'A'
}

func worktreeStatusUnmerged(code string) bool {
	switch code {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return false
	}
}

func worktreeIndexStaged(column byte) bool {
	switch column {
	case 'A', 'M', 'D', 'R', 'C', 'T':
		return true
	default:
		return false
	}
}

func worktreeColumnModified(column byte) bool {
	switch column {
	case 'M', 'D', 'T':
		return true
	default:
		return false
	}
}
