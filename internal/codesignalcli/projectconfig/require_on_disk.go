package projectconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// RequireOnDisk resolves the repository-relative path against the repository
// root rather than the directory the scan started in. A malformed path is left
// for Load to report, which keeps that message unchanged. It checks what is at
// the path: only a regular file reached without crossing a symbolic link that
// the process can open passes, and a state that cannot be told apart from "the
// file is there" is refused. It opens the file to prove it is readable but does
// not read its bytes, so a file whose content is not a valid policy is still
// judged from the analyzed revision.
func RequireOnDisk(dir, repoPath string) error {
	if ValidatePath(repoPath) != nil {
		return nil
	}
	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return err
	}
	return checkRegularFileUnder(root, repoPath)
}

// checkRegularFileUnder walks the path through one Root handle, so a name that
// escapes the root is an error and the check follows the same rule as a read.
func checkRegularFileUnder(rootDir, repoPath string) error {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return unusableOnDiskError(repoPath, "cannot be checked on disk: "+causeText(err))
	}
	defer func() { _ = root.Close() }()
	if err := checkAncestors(root, repoPath); err != nil {
		return err
	}
	if err := checkEntry(root, repoPath); err != nil {
		return err
	}
	return checkExactNames(root, repoPath)
}

// checkAncestors requires every parent of the policy to be a real directory.
func checkAncestors(root *os.Root, repoPath string) error {
	parts := strings.Split(repoPath, "/")
	for i := 1; i < len(parts); i++ {
		ancestor := strings.Join(parts[:i], "/")
		info, err := root.Lstat(filepath.FromSlash(ancestor))
		if err != nil {
			return classifyStatError(repoPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return unusableOnDiskError(repoPath, fmt.Sprintf("is reached through a symbolic link, %q", ancestor))
		}
		if !info.IsDir() {
			return blockedParentError(repoPath, ancestor)
		}
	}
	return nil
}

// checkEntry requires the policy itself to be a regular file and not a link,
// and a file this process is allowed to open.
func checkEntry(root *os.Root, repoPath string) error {
	info, err := root.Lstat(filepath.FromSlash(repoPath))
	if err != nil {
		return classifyStatError(repoPath, err)
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return unusableOnDiskError(repoPath, "is a symbolic link, not a regular file")
	case !info.Mode().IsRegular():
		return unusableOnDiskError(repoPath, "is not a regular file")
	}
	return checkReadable(root, repoPath)
}
