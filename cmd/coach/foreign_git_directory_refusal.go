package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// refuseForeignGitDirectory reports a scan that git's repository cannot hold, and the exit code it ends with. A
// git directory that names another repository is a mistake in how the scan was started and exits 2; a question git
// could not answer is an operational failure and exits 1.
func refuseForeignGitDirectory(dir string, stderr *os.File) (exitCode int, refused bool) {
	err := gitrepo.RefuseForeignGitDirectory(gitrepo.RunBytes, dir)
	if err == nil {
		return 0, false
	}
	fmt.Fprintf(stderr, "coach codesignal: %s\n", err)
	var foreign *gitrepo.ForeignGitDirectoryError
	if errors.As(err, &foreign) {
		return 2, true
	}
	return 1, true
}
