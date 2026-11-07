//go:build unix

package projectconfig

import "syscall"

// openWithoutWaiting is the open flag that keeps opening a FIFO from waiting for
// a writer.
const openWithoutWaiting = syscall.O_NONBLOCK
