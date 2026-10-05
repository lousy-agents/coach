// Package gitfixture builds throwaway Git repositories for codesignalcli
// tests: one initialized repository per test, and one commit per file.
package gitfixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// T is the subset of testing.T and GinkgoT() the fixtures report through.
type T interface {
	Helper()
	Fatalf(format string, args ...any)
	TempDir() string
}

// CommitEnv pins a commit identity so a fixture commit never depends on the
// host's git configuration.
var CommitEnv = append(os.Environ(),
	"GIT_AUTHOR_NAME=coach-test",
	"GIT_AUTHOR_EMAIL=coach-test@example.com",
	"GIT_COMMITTER_NAME=coach-test",
	"GIT_COMMITTER_EMAIL=coach-test@example.com",
)

// Init runs `git init` in a fresh temporary directory and returns it.
func Init(t T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, nil, "init")
	return dir
}

// CommitFile writes contents to name under dir, commits it alone, and
// returns the new HEAD commit SHA.
func CommitFile(t T, dir, name, contents string) string {
	t.Helper()
	target := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	run(t, dir, nil, "add", name)
	run(t, dir, CommitEnv, "commit", "-m", "commit "+name)
	return strings.TrimSpace(run(t, dir, nil, "rev-parse", "HEAD"))
}

// Rename moves from to to with `git mv` and commits the rename.
func Rename(t T, dir, from, to string) {
	t.Helper()
	run(t, dir, nil, "mv", from, to)
	run(t, dir, CommitEnv, "commit", "-m", "rename "+from+" to "+to)
}

func run(t T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, stderr)
	}
	return string(output)
}
