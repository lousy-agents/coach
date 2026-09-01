//go:build windows

package codesignalcli

import (
	"os"

	"golang.org/x/sys/windows"
)

// isTerminal reports whether f's file descriptor is a real console by
// calling GetConsoleMode: it succeeds only for an actual console handle,
// unlike an os.ModeCharDevice check, which is also true for non-console
// character devices such as the OS's null device.
func isTerminal(f *os.File) bool {
	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode)
	return err == nil
}
