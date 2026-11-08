package projectconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func makeCanonicalParent(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func writeCanonical(t *testing.T, repo string) {
	t.Helper()
	makeCanonicalParent(t, repo)
	writeFile(t, repo, CanonicalPath, "{}")
}

func makeCanonicalDirectory(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, CanonicalPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func linkCanonical(t *testing.T, repo string) {
	t.Helper()
	makeCanonicalParent(t, repo)
	if err := os.Symlink("../outside.json", filepath.Join(repo, CanonicalPath)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

func fileInPlaceOfTheDirectory(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, repo, ".lousy-agents", "a file in the way")
}

func unsearchableParent(t *testing.T, repo string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root can search any directory")
	}
	writeCanonical(t, repo)
	parent := filepath.Join(repo, ".lousy-agents")
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
}
