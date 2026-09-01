//go:build linux

package codesignalcli

import (
	"fmt"
	"os"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// openPTYSlave opens a real Linux pseudo-terminal pair via /dev/ptmx and
// returns the slave end, which is a genuine controlling terminal -- the
// only way to give TestHasControllingTerminal_PTY discriminating power
// against a stub that always returns false. The caller closes the
// returned file; the master end is closed automatically via t.Cleanup.
func openPTYSlave(t *testing.T) *os.File {
	t.Helper()

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("open /dev/ptmx: %v (no pty support in this sandbox)", err)
	}
	t.Cleanup(func() { master.Close() })

	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Skipf("TIOCSPTLCK: %v", err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Skipf("TIOCGPTN: %v", err)
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", n)
	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("open %s: %v", slavePath, err)
	}
	return slave
}
