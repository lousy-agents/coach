package projectconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRegularFileUnderRefusesAPathThatClimbsOutOfTheRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, repoPath := range []string{"../outside.json", "sub/../../outside.json"} {
		t.Run(repoPath, func(t *testing.T) {
			assertRefusedOutsideRoot(t, root, repoPath)
		})
	}
}

func assertRefusedOutsideRoot(t *testing.T, root, repoPath string) {
	t.Helper()
	if err := checkRegularFileUnder(root, repoPath); err == nil {
		t.Fatalf("checkRegularFileUnder accepted %q, a regular file outside the root, as the policy", repoPath)
	}
}
