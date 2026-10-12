// Package sourcescope labels each selected file as production, test-only,
// excluded, or unknown from the analyzed revision's own Go build graph and
// tsconfig.json, and filters or tallies files by that label.
package sourcescope

import (
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	Production = "production"
	TestOnly   = "test_only"
	Excluded   = "excluded"
	Unknown    = "unknown"
)

// tallyClassified splits classified (files already labeled by
// classifySourceFiles) into files that ship (kept) and files that don't
// (excluded), grouped by (SourceScope reason, Language) pair. It is shared
// by Apply and ApplyBaseline, whose only difference is
// what they do with the two results.
func tallyClassified(classified []gitrepo.SelectedFile) (kept []gitrepo.SelectedFile, excluded []codesignal.CoverageGroup) {
	type groupKey struct{ reason, language string }
	counts := make(map[groupKey]int)

	kept = make([]gitrepo.SelectedFile, 0, len(classified))
	for _, file := range classified {
		if file.SourceScope == TestOnly || file.SourceScope == Excluded {
			counts[groupKey{reason: file.SourceScope, language: string(file.Language)}]++
			continue
		}
		kept = append(kept, file)
	}

	keys := make([]groupKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].reason != keys[j].reason {
			return keys[i].reason < keys[j].reason
		}
		return keys[i].language < keys[j].language
	})

	for _, key := range keys {
		excluded = append(excluded, codesignal.CoverageGroup{
			Reason:   key.reason,
			Language: key.language,
			Count:    counts[key],
		})
	}

	return kept, excluded
}

// ApplyBaseline labels each selected file according to the
// source set it belongs to, same as Apply, but instead of
// silently dropping test_only/excluded files it tallies them into excluded,
// grouped by (SourceScope reason, Language) pair, so a Repository Baseline
// report can record what was left out and why. When scope is "all",
// nothing is excluded, matching Apply's "all" semantics.
func ApplyBaseline(dir, revisionSHA, buildTarget, scope string, files []gitrepo.SelectedFile) (kept []gitrepo.SelectedFile, excluded []codesignal.CoverageGroup, err error) {
	classified, err := classifySourceFiles(dir, revisionSHA, buildTarget, scope, files)
	if err != nil {
		return nil, nil, err
	}
	if scope == "all" {
		return classified, nil, nil
	}

	kept, excluded = tallyClassified(classified)
	return kept, excluded, nil
}

// Apply labels each selected file according to the source set it
// belongs to, then splits out files known not to ship when scope is not
// "all" into excluded, grouped by (SourceScope reason, Language) pair, so
// the diff flow can record what was left out and why. Unknown files are
// deliberately retained in kept so an incomplete project configuration
// cannot silently hide a finding.
func Apply(dir, headSHA, buildTarget, scope string, files []gitrepo.SelectedFile) (kept []gitrepo.SelectedFile, excluded []codesignal.CoverageGroup, err error) {
	classified, err := classifySourceFiles(dir, headSHA, buildTarget, scope, files)
	if err != nil {
		return nil, nil, err
	}
	if scope == "all" {
		return classified, nil, nil
	}

	kept, excluded = tallyClassified(classified)
	return kept, excluded, nil
}
