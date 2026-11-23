package projectconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// DiskSelection is the policy the working tree holds: the bytes read and the digest of those same bytes. The zero
// value is no policy on disk.
type DiskSelection struct {
	Path, Digest string
	Content      json.RawMessage
}

// SelectOnDisk is the policy file the working tree selects for a scan, with the digest of the bytes it holds now.
// An empty namedPath means the policy was discovered: the canonical file if something is there, else project.json.
// A non-empty namedPath, the --project-config value, is that path alone. A path with nothing at it selects nothing and
// the next candidate is tried. An entry that cannot be the policy (a link, a directory, a pipe, a path below a file, a
// file that cannot be read or is over MaxBytes) is a *DiskPolicyRefusal that stops the search with nothing selected, so
// project.json never stands in for a canonical path that is there and unusable. Which refusals matter is for the
// caller, which knows what the revision commits: an uncommitted project.json is not a policy to refuse. It asks git only
// for the repository root, so it changes neither the index nor the working tree. Any other error is a failure of the
// environment and not a verdict about the file.
func SelectOnDisk(dir, namedPath string) (DiskSelection, error) {
	return selectOnDiskThrough(NewGitRunner(), dir, namedPath)
}

func selectOnDiskThrough(run gitrepo.Runner, dir, namedPath string) (DiskSelection, error) {
	rootDir, err := gitrepo.RepositoryRootWith(run, dir)
	if err != nil {
		return DiskSelection{}, err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return DiskSelection{}, fmt.Errorf("opening the repository root: %s", causeText(err))
	}
	defer func() { _ = root.Close() }()
	for _, candidate := range onDiskCandidates(namedPath) {
		selection, found, err := selectRegularFile(root, candidate)
		if err != nil || found {
			return selection, err
		}
	}
	return DiskSelection{}, nil
}

func onDiskCandidates(namedPath string) []string {
	if namedPath != "" {
		return []string{namedPath}
	}
	return []string{CanonicalPath, DefaultPath}
}

func selectRegularFile(root *os.Root, repoPath string) (DiskSelection, bool, error) {
	if err := checkRegularFileThrough(osDiskRoot{root}, repoPath); err != nil {
		return DiskSelection{}, false, refusalOfEntry(root, repoPath, err)
	}
	content, err := readBounded(root, repoPath)
	if err != nil {
		return DiskSelection{}, false, refusalOfRead(repoPath, err)
	}
	return DiskSelection{Path: repoPath, Digest: Digest(content), Content: content}, true, nil
}

// refusalOfEntry is the refusal for a path the disk check stopped at, or nil for a path with nothing at it, which
// selects nothing without refusing. A failure of the check that is no verdict about the path is returned as it is.
func refusalOfEntry(root *os.Root, repoPath string, err error) error {
	var verdict *ConfigError
	switch {
	case !errors.As(err, &verdict):
		return err
	case verdict.AbsentFromDisk():
		return nil
	}
	reason, known := entryRefusalReasons[verdict.PolicyCause]
	if !known {
		reason = refusalUnreadable
	}
	return &DiskPolicyRefusal{Path: repoPath, Reason: reason, Directory: reason == refusalNotRegular && isDirectory(root, repoPath)}
}

func isDirectory(root *os.Root, repoPath string) bool {
	info, err := root.Lstat(filepath.FromSlash(repoPath))
	return err == nil && info.IsDir()
}

// refusalOfRead is the refusal for a file that passed the check and then could not be read in full within the budget.
func refusalOfRead(repoPath string, err error) error {
	var oversize *oversizePolicyError
	if errors.As(err, &oversize) {
		return &DiskPolicyRefusal{Path: repoPath, Reason: refusalOversize}
	}
	return &DiskPolicyRefusal{Path: repoPath, Reason: refusalUnreadable}
}
