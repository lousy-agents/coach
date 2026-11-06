package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// computeChangedRanges diffs path from the merge base to HEAD. Color is off on
// the command line because a configured color.diff=always would put escape codes
// in front of the hunk headers and the parse would find no ranges.
func computeChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, *codesignal.Diagnostic) {
	args := []string{"-c", "color.ui=false", "diff", "--no-color", "--unified=0", "--no-ext-diff", "--no-textconv", mergeBaseSHA, "HEAD"}
	output, err := gitrepo.RunBytes(dir, append(args, "--", path)...)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("computing changed ranges for %q: %s", path, err),
		}
	}

	ranges, err := parseChangedRanges(output)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("parsing diff for %q: %s", path, err),
		}
	}
	return ranges, nil
}
