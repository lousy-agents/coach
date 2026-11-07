//go:build !unix

package projectconfig

// openWithoutWaiting is zero where there is no FIFO to wait on.
const openWithoutWaiting = 0
