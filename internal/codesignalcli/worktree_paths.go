package codesignalcli

import (
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func (s worktreePathSets) diagnostics() []codesignal.Diagnostic {
	sort.Strings(s.untracked)
	sort.Strings(s.staged)
	sort.Strings(s.modified)
	sort.Strings(s.unmerged)

	var diagnostics []codesignal.Diagnostic
	diagnostics = appendCategory(diagnostics, "untracked", s.untracked)
	diagnostics = appendCategory(diagnostics, "staged", s.staged)
	diagnostics = appendCategory(diagnostics, "modified", s.modified)
	diagnostics = appendCategory(diagnostics, "unmerged", s.unmerged)
	if len(diagnostics) > 0 {
		return diagnostics
	}
	return unsupportedOnlyDiagnostic(s.unsupported)
}

type worktreePathSets struct {
	untracked   []string
	staged      []string
	modified    []string
	unmerged    []string
	unsupported []string
}

func collectWorktreePaths(entries []gitrepo.WorktreeEntry) worktreePathSets {
	var sets worktreePathSets
	for _, entry := range entries {
		sets.add(entry)
	}
	return sets
}

func (s *worktreePathSets) add(entry gitrepo.WorktreeEntry) {
	if entry.Path == "" {
		return
	}
	untracked, staged, modified, unmerged := gitrepo.ClassifyWorktreeStatus(entry.Code)
	if !untracked && !staged && !modified && !unmerged {
		return
	}
	if !supportedWorktreePath(entry.Path) {
		s.unsupported = append(s.unsupported, entry.Path)
		return
	}
	s.appendSupported(entry.Path, untracked, staged, modified, unmerged)
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
