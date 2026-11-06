package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// fsmonitorMarker writes a core.fsmonitor program that leaves a marker file when git runs it, and
// returns the program's path and the marker's.
func fsmonitorMarker(t *testing.T) (program, marker string) {
	t.Helper()
	scripts := t.TempDir()
	marker = filepath.Join(scripts, "fsmonitor-ran")
	program = filepath.Join(scripts, "fsmonitor.sh")
	body := "#!/bin/sh\ntouch '" + marker + "'\nprintf '\\0'\n"
	if err := os.WriteFile(program, []byte(body), 0o755); err != nil {
		t.Fatalf("write fsmonitor program: %v", err)
	}
	return program, marker
}

func TestGitReadsDoNotRunACoreFsmonitorProgram(t *testing.T) {
	seams := []struct {
		name string
		read func(dir string) error
	}{
		{"the working-tree status read", func(dir string) error { _, err := WorktreeStatus(dir); return err }},
		{"the bounded runner other reads use", func(dir string) error {
			_, err := BoundedRunner(1<<20, 1<<16, 30*time.Second)(dir, "status", "--porcelain=v1", "-z")
			return err
		}},
		{"the unbounded git read", func(dir string) error { _, err := RunBytes(dir, "status", "--porcelain=v1", "-z"); return err }},
	}
	sources := []struct {
		name  string
		setup func(t *testing.T, dir, program string)
	}{
		{"the repository's own config", func(t *testing.T, dir, program string) {
			if out, err := exec.Command("git", "-C", dir, "config", "core.fsmonitor", program).CombinedOutput(); err != nil {
				t.Fatalf("git config: %v: %s", err, out)
			}
		}},
		{"the environment's injected config", func(t *testing.T, _, program string) {
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
			t.Setenv("GIT_CONFIG_VALUE_0", program)
		}},
	}
	for _, seam := range seams {
		for _, source := range sources {
			t.Run(seam.name+" under "+source.name, func(t *testing.T) {
				dir := gitfixture.Init(t)
				gitfixture.CommitFile(t, dir, "a.go", "package a\n")
				program, marker := fsmonitorMarker(t)
				source.setup(t, dir, program)

				if err := seam.read(dir); err != nil {
					t.Fatalf("read: %v", err)
				}

				if _, err := os.Stat(marker); err == nil {
					t.Errorf("git ran the core.fsmonitor program from %s: scanning a repository must not execute code its configuration names", source.name)
				}
			})
		}
	}
}
