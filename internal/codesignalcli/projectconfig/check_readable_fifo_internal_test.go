//go:build unix

package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCheckReadableRefusesAFifoInsteadOfBlockingOnIt(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pol.json")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer func() { _ = root.Close() }()

	result := make(chan error, 1)
	go func() { result <- checkReadable(root, "pol.json") }()

	select {
	case err := <-result:
		var configErr *ConfigError
		if !errors.As(err, &configErr) || configErr.Kind != KindInvalid || !strings.Contains(configErr.Message, "is not a regular file") {
			t.Errorf("checkReadable = %v; want a project_config_invalid error saying the entry is not a regular file", err)
		}
	case <-time.After(3 * time.Second):
		releaseFifoReader(fifo)
		t.Fatal("checkReadable blocked opening a FIFO that nothing writes to, which a swap between its stat and its open would hang a scan on")
	}
}

// releaseFifoReader opens the write side so a reader stuck opening the FIFO
// returns and the test does not leak it.
func releaseFifoReader(fifo string) {
	writer, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err == nil {
		_ = writer.Close()
	}
}
