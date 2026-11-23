package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// copyRepository copies .git, the files git tracks and the untracked files it
// does not ignore, from the working directory into target, so the copy holds
// the tree a developer is testing, as a checkout CI's runner user owns.
func copyRepository(target string) error {
	out, err := gitrepo.RunBytes(".", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("listing the repository's files: %v", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if copied, err := exec.Command("cp", "-a", ".git", filepath.Join(target, ".git")).CombinedOutput(); err != nil {
		return fmt.Errorf("copying .git: %v: %s", err, copied)
	}
	for _, name := range bytes.Split(bytes.TrimRight(out, "\x00"), []byte{0}) {
		if err := copyEntry(string(name), filepath.Join(target, string(name))); err != nil {
			return err
		}
	}
	return nil
}
