//go:build linux || solaris

package codesignalcli

import "golang.org/x/sys/unix"

// ioctlReadTermios is the termios-read ioctl request number on Linux and
// Solaris, consumed by isTerminal in controlling_terminal_unix.go.
const ioctlReadTermios = unix.TCGETS
