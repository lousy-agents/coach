package main

import (
	"bytes"
	"fmt"
	"io"
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

func copyEntry(source, target string) error {
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(link, target)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return copyFile(source, target, info.Mode().Perm())
}

func copyFile(source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
