package gitrepo

import (
	"path/filepath"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// WithUnsupportedFile adds one unsupported-language file to coverage: it is
// discovered and counted under its extension, in the extension order
// DiscoverTrackedFiles lists the groups in. The slices are rebuilt, never edited
// in place.
func WithUnsupportedFile(coverage codesignal.Coverage, path string) codesignal.Coverage {
	counts := map[string]int{coverageLanguageLabel(filepath.Ext(path)): 1}
	for _, group := range coverage.Unsupported {
		counts[group.Language] += group.Count
	}
	coverage.Unsupported = unsupportedGroups(counts)
	coverage.TrackedFilesDiscovered++
	return coverage
}
