package worktreeoverlay_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

func TestOSDiskDoesNotFollowASymbolicLinkInAParentDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "h.go"), []byte("package outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "pkg")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	disk := worktreeoverlay.OSDisk{Root: root}

	info, err := disk.Lstat("pkg/h.go")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Lstat(pkg/h.go) mode = %v; a path under a symlinked directory must read as a symlink, not as the outside file", info.Mode())
	}
	if data, err := disk.ReadFile("pkg/h.go", info); err == nil {
		t.Errorf("ReadFile returned %q from outside the repository through a symlinked parent", data)
	}
}

func TestOSDiskNeverBlocksOnAFIFO(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "p.go")
	if output, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Skipf("mkfifo unavailable: %v %s", err, output)
	}
	disk := worktreeoverlay.OSDisk{Root: root}
	result := make(chan error, 1)

	examined, err := disk.Lstat("p.go")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_, err := disk.ReadFile("p.go", examined)
		result <- err
	}()

	select {
	case err := <-result:
		if !errors.Is(err, worktreeoverlay.ErrNotRegular) {
			t.Errorf("ReadFile(fifo) error = %v, want ErrNotRegular", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReadFile blocked on a FIFO: a file swapped in between the check and the read would hang the scan")
	}
}
