package codesignalcli

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// fileSide is one side of a changed file as a read of it names it: the revision the content comes from, and
// the kind of the diagnostic for content past the limit on one file at that place. git show holds a read to
// 32 MiB and the analyzer holds a file to 2 MiB; v0.11.0 named a file too large at every size, so a consumer
// that matches on the kind finds the same one whichever limit stopped the read. The kind is the one the
// analyzer's own refusal gets there (a file at the head or a removed file is file_too_large, the base side
// of a modified file is base_analysis_failed).
type fileSide struct {
	revision     string // "head" or "base"
	tooLargeKind string
}

var (
	headSide         = fileSide{revision: "head", tooLargeKind: "file_too_large"}
	removedBaseSide  = fileSide{revision: "base", tooLargeKind: "file_too_large"}
	modifiedBaseSide = fileSide{revision: "base", tooLargeKind: "base_analysis_failed"}
)

// readFailure turns a failed read of one side of a file into its diagnostic, or, when git did not answer in
// time, into the error that ends the scan: a git that does not answer will not answer for the next file
// either, and a scan that waited out its budget once per file would take that many budgets to give up.
func readFailure(path string, side fileSide, err error) ([]codesignal.Diagnostic, error) {
	if stalled := gitrepo.TimedOut(err); stalled != nil {
		return nil, stalled
	}
	kind := side.revision + "_read_failed"
	if printedPastItsBudget(err) {
		kind = side.tooLargeKind
	}
	return []codesignal.Diagnostic{{
		Path:    path,
		Kind:    kind,
		Message: fmt.Sprintf("reading %s content for %q: %s", side.revision, path, err),
	}}, nil
}

// printedPastItsBudget is true for a read that git answered with more than the read may hold.
func printedPastItsBudget(err error) bool {
	var bound *gitrepo.BoundError
	return errors.As(err, &bound) && bound.Kind == gitrepo.BoundStdout
}
