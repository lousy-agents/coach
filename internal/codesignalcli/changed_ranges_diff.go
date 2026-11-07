package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// computeChangedRanges reads the changed line ranges of one file. Its error is a git that did not answer in
// time (see readFailure); a diff that cannot be read or parsed is a diagnostic.
func computeChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, *codesignal.Diagnostic, error) {
	// A patch is where git runs the programs a repository or the environment names for one: an external
	// diff program (--no-ext-diff) and a textconv driver that .gitattributes selects (--no-textconv). The
	// hunk headers this reads do not need either, and a read-only scan must not execute them.
	output, err := runGit(dir, "diff", "--unified=0", "--no-ext-diff", "--no-textconv", mergeBaseSHA, "HEAD", "--", path)
	if stalled := gitrepo.TimedOut(err); stalled != nil {
		return nil, nil, stalled
	}
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("computing changed ranges for %q: %s", path, err),
		}, nil
	}

	ranges, err := parseChangedRanges(output)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("parsing diff for %q: %s", path, err),
		}, nil
	}
	return ranges, nil, nil
}
