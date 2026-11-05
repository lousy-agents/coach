package codesignalcli

import "github.com/lousy-agents/coach/pkg/codesignal"

// coverageForAbsentSource accounts for a supported source file the scan could
// not read: a file the working tree removed is no longer discovered, and any
// other absent file (a symbolic link, an unreadable or racing file) is
// unanalyzable. Either way discovered stays equal to the sum of its parts.
func (v *WorktreeView) coverageForAbsentSource(coverage codesignal.Coverage, name string) codesignal.Coverage {
	if v.removed[name] {
		coverage.TrackedFilesDiscovered--
		return coverage
	}
	coverage.FilesUnanalyzable++
	return coverage
}
