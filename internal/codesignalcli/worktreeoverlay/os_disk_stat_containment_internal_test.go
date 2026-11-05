package worktreeoverlay

import (
	"os"
	"path/filepath"
	"testing"
)

// Lstat must stay inside the root on its own. The overlay's path validation
// stops ".." today, but a stat that leaves the root leaks existence, size and
// mode of an outside file whenever that guard is bypassed.
func TestOSDiskLstatNeverReportsAnythingOutsideTheRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(parent, "linkdir")); err != nil {
		t.Fatal(err)
	}
	disk := OSDisk{Root: root}

	for _, name := range []string{"../outside.txt", "../linkdir/x", "../linkdir"} {
		if info, err := disk.Lstat(name); err == nil {
			t.Errorf("Lstat(%q) returned metadata (mode %v, size %d) for a path outside the root", name, info.Mode(), info.Size())
		}
	}
}
