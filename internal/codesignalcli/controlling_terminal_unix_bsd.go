//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package codesignalcli

import "golang.org/x/sys/unix"

// ioctlReadTermios is the termios-read ioctl request number on the BSD
// family (including Darwin), consumed by isTerminal in
// controlling_terminal_unix.go. This differs from the Linux/Solaris value
// (unix.TCGETS); using the wrong constant fails to compile rather than
// misbehaving at runtime, since golang.org/x/sys/unix only defines each
// constant for the platforms that actually have it.
const ioctlReadTermios = unix.TIOCGETA
