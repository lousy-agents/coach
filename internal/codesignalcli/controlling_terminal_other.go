//go:build !linux && !solaris && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !windows

package codesignalcli

import "os"

// isTerminal has no ioctl-based tty probe on this platform (aix, hurd, js,
// plan9, wasip1, zos, and any future GOOS not covered by the sibling
// ioctl- or console-mode-based implementations -- note that android,
// ios, and illumos are covered by those siblings via Go's implicit
// linux/darwin/solaris GOOS constraints, respectively, despite not being
// named in this file's own build tag).
// It fails closed (always false) rather than falling back to an
// os.ModeCharDevice check, which would wrongly report a controlling
// terminal for non-terminal character devices such as the OS's null
// device -- exactly the failure mode HasControllingTerminal exists to
// avoid.
func isTerminal(f *os.File) bool {
	return false
}
