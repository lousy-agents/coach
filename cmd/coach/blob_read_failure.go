package main

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// The prefix each mode puts before a failure to read the repository, so that the user sees the command they ran.
const (
	checkProjectPrefix = "coach codesignal --check-project: "
	scanPrefix         = "coach codesignal: "
)

// wordBlobReadFailure is err with a failure to read a blob at the analyzed revision worded for the mode that
// hit it. The repository layer returns the failure typed and names no command; this is where the command is
// named, and where an object the repository lacks is told in the one sentence projectconfig owns for it. An
// error that is not such a failure is returned as it is.
func wordBlobReadFailure(run gitrepo.Runner, dir string, err error, prefix string) error {
	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		return err
	}
	var read *gitrepo.BlobReadError
	if !errors.As(operational.Cause, &read) {
		return err
	}
	message := operational.Message
	var absent *gitrepo.ObjectNotLocalError
	if errors.As(read, &absent) && read.Path != "" {
		message = fmt.Sprintf("%q could not be read at revision %q: %s", read.Path, read.Revision, projectconfig.ObjectNotLocalWords(run, dir, read.Revision, read.Path))
	}
	return &gitrepo.OperationalError{Message: prefix + message, Cause: operational.Cause}
}
