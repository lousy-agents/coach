package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func computeChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, *codesignal.Diagnostic) {
	// A patch is where git runs the programs a repository or the environment names for one: an external
	// diff program (--no-ext-diff) and a textconv driver that .gitattributes selects (--no-textconv). The
	// hunk headers this reads do not need either, and a read-only scan must not execute them.
	output, err := gitrepo.RunBytes(dir, "diff", "--unified=0", "--no-ext-diff", "--no-textconv", mergeBaseSHA, "HEAD", "--", path)
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
