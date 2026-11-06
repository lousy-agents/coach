package projectconfig

import (
	"errors"
	"io/fs"
	"syscall"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// causeText is the operating system's reason for a failed stat and nothing
// else. The error's own text repeats the path it was given, which can hold
// control characters that would reach the terminal.
func causeText(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno.Error()
	}
	return "unexpected file-system error"
}

// classifyStatError is the policy error for a stat that failed: a path that is
// not there is missing, and anything else is a path that cannot be checked.
func classifyStatError(repoPath string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return missingError(repoPath)
	}
	return unusableOnDiskError(repoPath, projectreadiness.CauseUnreadableOnDisk, "cannot be checked on disk: "+causeText(err))
}

// classifyOpenError is classifyStatError for an open that failed on an entry
// the stat had just found: what is wrong is that it cannot be read.
func classifyOpenError(repoPath string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return missingError(repoPath)
	}
	return unusableOnDiskError(repoPath, projectreadiness.CauseUnreadableOnDisk, "cannot be read on disk: "+causeText(err))
}
