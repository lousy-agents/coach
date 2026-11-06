//go:build unix

package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A directory swapped for a FIFO after the stat that found a directory would make an open wait for a
// writer that never comes. The listing has to end, with a refusal, whatever the entry has become.
func TestSpellingInDoesNotWaitForAWriterWhenTheDirectoryHasBecomeAFifo(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "cfg")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	t.Cleanup(func() { releaseWaitingOpen(fifo) })

	done := make(chan error, 1)
	go func() {
		_, err := spellingIn(root, "cfg", "p.json")
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, syscall.ENOTDIR) {
			t.Errorf("spellingIn on a FIFO = %v; want an error that says it is not a directory", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("spellingIn is still waiting for a writer on a FIFO that stands where a directory was")
	}
}

// releaseWaitingOpen lets an open that is waiting for a writer return, so a failing test leaves no
// goroutine stuck in the kernel.
func releaseWaitingOpen(fifo string) {
	if writer, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
		_ = writer.Close()
	}
}
