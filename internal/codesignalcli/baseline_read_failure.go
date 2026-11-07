package codesignalcli

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// baselineReadFailure turns a failed read of one file of a --baseline scan into its diagnostic. A file past
// the limit on one file is the same kind of finding the analyzer gives a file past its own limit, so that a
// consumer sees one kind for it. A git that did not answer in time ends the scan (see readFailure).
func baselineReadFailure(path string, err error) ([]codesignal.Diagnostic, error) {
	var tooLarge *gitrepo.ObjectTooLargeError
	if errors.As(err, &tooLarge) {
		return []codesignal.Diagnostic{{Path: path, Kind: "file_too_large", Message: tooLarge.Error()}}, nil
	}
	return readFailure(path, headSide, err)
}
