package codesignalcli

type worktreePathSets struct {
	untracked   []string
	staged      []string
	modified    []string
	unmerged    []string
	unsupported []string
}

func collectWorktreePaths(entries []worktreeStatusEntry) worktreePathSets {
	var sets worktreePathSets
	for _, entry := range entries {
		sets.add(entry)
	}
	return sets
}

func (s *worktreePathSets) add(entry worktreeStatusEntry) {
	if entry.path == "" {
		return
	}
	untracked, staged, modified, unmerged := classifyWorktreeStatus(entry.code)
	if !untracked && !staged && !modified && !unmerged {
		return
	}
	if !supportedWorktreePath(entry.path) {
		s.unsupported = append(s.unsupported, entry.path)
		return
	}
	s.appendSupported(entry.path, untracked, staged, modified, unmerged)
}

func (s *worktreePathSets) appendSupported(path string, untracked, staged, modified, unmerged bool) {
	if untracked {
		s.untracked = append(s.untracked, path)
	}
	if staged {
		s.staged = append(s.staged, path)
	}
	if modified {
		s.modified = append(s.modified, path)
	}
	if unmerged {
		s.unmerged = append(s.unmerged, path)
	}
}
