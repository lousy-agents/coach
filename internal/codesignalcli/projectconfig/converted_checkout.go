package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// holdsTheCommittedBlobConverted is whether the regular file at repoPath under the worktree rooted at root, whose bytes
// are not the committed blob's, is that blob once git's own conversions are applied to it: the line endings that
// core.autocrlf and the text and eol attributes change, $Id$ expansion and working-tree-encoding. A checkout made on
// Windows holds converted bytes with no edit at all, and git status calls it clean, so it is not a file with work in it.
//
// Git applies the conversion for the path when it hashes the file, which asks it nothing it writes: the index is not
// refreshed, no lock is taken and no hook runs. A filter attribute selects a program, and no program runs on a scan, so a
// file that has one selected is not converted and is taken to differ; git is asked for the attribute first, and the
// hash is asked only when none is set. Anything git cannot answer is taken to differ, because a replacement offered
// over a file that was not understood can destroy what is in it.
func holdsTheCommittedBlobConverted(run gitrepo.Runner, root, repoPath, objectID string) bool {
	if !isARegularFileUnder(root, repoPath) {
		return false
	}
	if selected, err := aFilterIsSelectedFor(run, root, repoPath); err != nil || selected {
		return false
	}
	output, err := run(root, "hash-object", "--path="+repoPath, "--", filepath.Join(root, filepath.FromSlash(repoPath)))
	return err == nil && strings.TrimSpace(string(output)) == objectID
}

// aFilterIsSelectedFor is whether the filter attribute of repoPath names a program to run. Unset, and not set at all,
// name none. The question is asked from the root of the repository, because check-attr takes a path relative to the
// directory it runs in.
func aFilterIsSelectedFor(run gitrepo.Runner, root, repoPath string) (bool, error) {
	output, err := run(root, "check-attr", "-z", "filter", "--", repoPath)
	if err != nil {
		return false, err
	}
	fields := strings.Split(string(output), "\x00")
	if len(fields) < 3 {
		return false, errors.New("git check-attr printed a record this version cannot read")
	}
	return fields[2] != "unspecified" && fields[2] != "unset", nil
}

// isARegularFileUnder is whether repoPath, reached through the root without crossing a link out of it, is a regular file
// and not a link.
func isARegularFileUnder(rootDir, repoPath string) bool {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat(filepath.FromSlash(repoPath))
	return err == nil && info.Mode().IsRegular()
}
