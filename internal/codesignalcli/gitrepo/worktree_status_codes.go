package gitrepo

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
