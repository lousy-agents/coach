//go:build !linux

package codesignalcli

import (
	"os"
	"testing"
)

// openPTYSlave has no portable pseudo-terminal helper outside Linux, so
// TestHasControllingTerminal_PTY skips rather than asserting a fabricated
// result.
func openPTYSlave(t *testing.T) *os.File {
	t.Helper()
	t.Skip("pty-backed true case is only exercised on linux")
	return nil
}
