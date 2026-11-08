package gitfixture

import (
	"os"
	"path/filepath"
	"strings"
)

// CommitLink commits a symbolic link named name that points at target, and returns the new HEAD
// commit SHA. It records the link in the index and the commit and leaves the working tree alone,
// so a test chooses what is on disk at that path.
func CommitLink(t T, dir, name, target string) string {
	t.Helper()
	return commitIndexEntry(t, dir, "120000", name, target)
}

// CommitGitlink commits a submodule entry named name, as a superproject records one, without a
// .gitmodules file or a checkout, and returns the new HEAD commit SHA.
func CommitGitlink(t T, dir, name string) string {
	t.Helper()
	head := strings.TrimSpace(run(t, dir, nil, "rev-parse", "HEAD"))
	run(t, dir, nil, "update-index", "--add", "--cacheinfo", "160000,"+head+","+name)
	run(t, dir, CommitEnv, "commit", "-m", "commit gitlink "+name)
	return strings.TrimSpace(run(t, dir, nil, "rev-parse", "HEAD"))
}

func commitIndexEntry(t T, dir, mode, name, contents string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(source, []byte(contents), 0o644); err != nil {
		t.Fatalf("write the object for %s: %v", name, err)
	}
	object := strings.TrimSpace(run(t, dir, nil, "hash-object", "-w", source))
	run(t, dir, nil, "update-index", "--add", "--cacheinfo", mode+","+object+","+name)
	run(t, dir, CommitEnv, "commit", "-m", "commit "+name)
	return strings.TrimSpace(run(t, dir, nil, "rev-parse", "HEAD"))
}
