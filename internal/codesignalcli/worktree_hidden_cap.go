package codesignalcli

import (
	"maps"
	"slices"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// maxRootsListed bounds the entries one reason adds to the report; the roots
// beyond it are counted in one further entry, so a repository with thousands of
// hidden directories does not turn the report into a listing of them.
const maxRootsListed = 20

// reasonDiagnostics gives reason one entry per root, up to maxRootsListed. The
// roots that hide in-scope source come first, so a reader sees them before the
// cap cuts the listing; name only orders roots of equal severity. The entry for
// the roots left out has no root, and says whether any of them hides in-scope
// source: the verdict of a reason cannot be settled by the roots it listed
// alone. An empty root key yields the root-less entry; callers pass at least
// one key.
func reasonDiagnostics(reason hiddenReason, verdicts map[string]string) []codesignal.Diagnostic {
	roots := orderRootsBySeverity(verdicts)
	listed := roots[:min(len(roots), maxRootsListed)]
	unlisted := roots[len(listed):]
	diagnostics := make([]codesignal.Diagnostic, 0, len(listed)+1)
	for _, root := range listed {
		diagnostics = append(diagnostics, reason.entry(root, verdicts[root]))
	}
	if len(unlisted) == 0 {
		return diagnostics
	}
	truncation := reason.entry("", mostSevereVerdict(unlisted, verdicts))
	truncation.RootsNotListed = len(unlisted)
	return append(diagnostics, truncation)
}

func orderRootsBySeverity(verdicts map[string]string) []string {
	roots := slices.Sorted(maps.Keys(verdicts))
	slices.SortStableFunc(roots, func(a, b string) int {
		return verdictRank(verdicts[a]) - verdictRank(verdicts[b])
	})
	return roots
}

func verdictRank(verdict string) int {
	switch verdict {
	case codesignal.HidesInScopeYes:
		return 0
	case codesignal.HidesInScopeNo:
		return 2
	default:
		return 1
	}
}

// mostSevereVerdict orders yes above unknown above no, and reads any value it
// does not know as unknown, the way a report written before the field existed
// reads.
func mostSevereVerdict(roots []string, verdicts map[string]string) string {
	severest := codesignal.HidesInScopeNo
	for _, root := range roots {
		switch verdicts[root] {
		case codesignal.HidesInScopeYes:
			return codesignal.HidesInScopeYes
		case codesignal.HidesInScopeNo:
		default:
			severest = codesignal.HidesInScopeUnknown
		}
	}
	return severest
}
