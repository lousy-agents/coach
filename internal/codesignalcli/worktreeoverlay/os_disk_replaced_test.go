package worktreeoverlay_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// replaceKeepingTheOldFile puts a different file at name, as a swap between the
// scan's stat and its open would. The examined file is renamed rather than
// deleted so its inode stays allocated: a new file cannot reuse it, so the two
// files are always distinguishable by identity.
func replaceKeepingTheOldFile(t *testing.T, root, name, contents string) {
	t.Helper()
	if err := os.Rename(filepath.Join(root, name), filepath.Join(root, name+".examined")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOSDiskReadFileRefusesAFileThatWasReplacedAfterItWasExamined(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // examined\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	disk := worktreeoverlay.OSDisk{Root: root}
	examined, err := disk.Lstat("a.go")
	if err != nil {
		t.Fatal(err)
	}
	replaceKeepingTheOldFile(t, root, "a.go", "package a // swapped in\n")

	data, err := disk.ReadFile("a.go", examined)

	if !errors.Is(err, worktreeoverlay.ErrReplaced) {
		t.Errorf("ReadFile = %q, %v; want ErrReplaced for a path that is a different file from the one examined", data, err)
	}
}

func TestOSDiskReadFileReadsTheFileThatWasExamined(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	disk := worktreeoverlay.OSDisk{Root: root}
	examined, err := disk.Lstat("a.go")
	if err != nil {
		t.Fatal(err)
	}

	data, err := disk.ReadFile("a.go", examined)

	if err != nil || string(data) != "package a\n" {
		t.Errorf("ReadFile = %q, %v; want the examined file's bytes", data, err)
	}
}
