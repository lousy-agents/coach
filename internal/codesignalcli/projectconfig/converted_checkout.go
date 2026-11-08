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
// file that has one is not converted and is taken to differ; git is asked for the attributes first, and the hash is asked
// only when there is no filter among them. Anything git cannot answer is taken to differ, because a replacement offered
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

// aFilterIsSelectedFor is whether repoPath has a filter attribute at all, with any value. check-attr prints a string value
// as it is written, so a filter driver that is named unset or unspecified prints what an attribute that is unset, or not
// set at all, prints; asking for the one attribute cannot tell them apart, and reading those two words as "no filter" hands
// git the hash with the driver's program still selected, which it then runs from the repository's configuration. So all the
// attributes of the path are listed: git leaves out the ones that are not specified, and any record for filter is a filter,
// including the one that is really unset, which is taken to have work in it as well and fails safe. The question is asked
// from the root of the repository, because check-attr takes a path relative to the directory it runs in.
func aFilterIsSelectedFor(run gitrepo.Runner, root, repoPath string) (bool, error) {
	output, err := run(root, "check-attr", "-z", "-a", "--", repoPath)
	if err != nil {
		return false, err
	}
	fields := strings.Split(string(output), "\x00")
	last := len(fields) - 1
	if fields[last] != "" || last%3 != 0 {
		return false, errors.New("git check-attr printed a record this version cannot read")
	}
	return namesAttribute(fields[:last], "filter"), nil
}

// namesAttribute is whether one of the records of git check-attr -z -a, each a path, an attribute and its value, is for
// the attribute.
func namesAttribute(records []string, attribute string) bool {
	for i := 1; i < len(records); i += 3 {
		if records[i] == attribute {
			return true
		}
	}
	return false
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
