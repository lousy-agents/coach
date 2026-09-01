//go:build linux || solaris || darwin || dragonfly || freebsd || netbsd || openbsd

package codesignalcli

import (
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal reports whether f's file descriptor is a real terminal by
// issuing the platform's termios-read ioctl (ioctlReadTermios): it succeeds
// only for an actual tty/pty device, unlike an os.ModeCharDevice check,
// which is also true for non-terminal character devices such as
// /dev/null. The ioctl number itself differs between the Linux/Solaris
// family and the BSD family (including Darwin), so it is supplied by a
// build-tag-selected constant in a sibling file rather than hard-coded
// here.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), ioctlReadTermios)
	return err == nil
}
