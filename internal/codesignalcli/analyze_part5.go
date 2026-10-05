package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func computeChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, *codesignal.Diagnostic) {
	output, err := gitrepo.RunBytes(dir, "diff", "--unified=0", "--no-ext-diff", mergeBaseSHA, "HEAD", "--", path)
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
