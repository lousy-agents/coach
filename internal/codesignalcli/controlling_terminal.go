package codesignalcli

import "os"

// HasControllingTerminal reports whether f is a genuine, interactive
// controlling terminal -- i.e. whether it is safe to prompt on it for
// guided policy authoring. It returns false for nil, for a
// closed or otherwise unstatable file, and for any file descriptor that
// does not respond to a terminal ioctl, including regular files, pipes,
// and character devices that are not terminals such as os.DevNull:
// os.ModeCharDevice alone is true for /dev/null and is not a valid proxy
// for "has a controlling terminal".
//
// This check only works on a real *os.File backed by an actual OS file
// descriptor: there is no injectable/fake mode for tests to force a true
// result, and stdio redirected in a test process (or under most CI
// runners) will report false even when it might behave like a terminal
// interactively elsewhere. Tests must exercise this via genuinely
// non-terminal files (regular files, os.DevNull, os.Pipe) and, where a
// true case is needed, a real pseudo-terminal, rather than by injecting a
// fake result.
func HasControllingTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	if _, err := f.Stat(); err != nil {
		return false
	}
	return isTerminal(f)
}
