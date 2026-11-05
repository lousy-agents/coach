package worktreeoverlay

import (
	"os"
	"path/filepath"
	"testing"
)

// A directory listing must stay inside the root on its own, as Lstat does: a
// listing that leaves the root reveals the names in an outside directory
// whenever the path validation in front of it is bypassed.
func TestOSDiskReadDirNamesNeverListsAnythingOutsideTheRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(parent, "outside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside", "secret.go"), []byte("package s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "outside"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	disk := OSDisk{Root: root}

	for _, dir := range []string{"../outside", "link"} {
		if names, err := disk.ReadDirNames(dir); err == nil {
			t.Errorf("ReadDirNames(%q) listed %v, a directory outside the root", dir, names)
		}
	}
}

func TestOSDiskReadDirNamesListsTheRootAndItsSubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	disk := OSDisk{Root: root}

	top, err := disk.ReadDirNames(".")
	if err != nil || len(top) != 1 || top[0] != "pkg" {
		t.Errorf("ReadDirNames(\".\") = %v, %v, want [pkg]", top, err)
	}
	inner, err := disk.ReadDirNames("pkg")
	if err != nil || len(inner) != 1 || inner[0] != "a.go" {
		t.Errorf("ReadDirNames(\"pkg\") = %v, %v, want [a.go]", inner, err)
	}
}
