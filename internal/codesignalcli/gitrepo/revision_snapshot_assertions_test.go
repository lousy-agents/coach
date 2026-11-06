package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
)

func assertSnapshotShape(t *testing.T, dir string) {
	t.Helper()
	assertExecutable(t, filepath.Join(dir, "bin", "run.sh"))
	assertSymlinkTo(t, filepath.Join(dir, "link"), "bin/run.sh")
	assertDirectory(t, filepath.Join(dir, "sub"))
	for _, name := range []string{"dir with space/f g.go", "same-a.txt", "nested/same-b.txt"} {
		assertPresent(t, filepath.Join(dir, filepath.FromSlash(name)))
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	if info, err := os.Stat(path); err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("%s = %v, %v; want an executable file", path, info, err)
	}
}

func assertSymlinkTo(t *testing.T, path, want string) {
	t.Helper()
	if target, err := os.Readlink(path); err != nil || target != want {
		t.Errorf("%s -> %q, %v; want a symbolic link to %s", path, target, err, want)
	}
}

func assertDirectory(t *testing.T, path string) {
	t.Helper()
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Errorf("%s = %v, %v; want a directory", path, info, err)
	}
}

func assertPresent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("%s is not in the snapshot: %v", path, err)
	}
}
