package projectconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// DiskSelection is the policy the working tree holds: the bytes read and the digest of those same bytes. The zero
// value is no policy on disk.
type DiskSelection struct {
	Path, Digest string
	Content      json.RawMessage
}

// SelectOnDisk is the policy file the working tree selects for a scan, with the digest of the bytes it holds now.
// An empty namedPath means the policy was discovered: the canonical file if that is a regular file, else project.json
// if that is. A non-empty namedPath, the --project-config value, is that path alone. A path that is absent, a link, or not a regular file is not selected,
// and a regular file that cannot be read in full within MaxBytes is an error with nothing selected. It asks git only
// for the repository root, so it changes neither the index nor the working tree. The error is for deciding, not for
// display: a refusal's wording can name --project-config even when the policy was discovered.
func SelectOnDisk(dir, namedPath string) (DiskSelection, error) {
	rootDir, err := gitrepo.RepositoryRootWith(NewGitRunner(), dir)
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
		if cannotBeThePolicy(err) {
			return DiskSelection{}, false, nil
		}
		return DiskSelection{}, false, err
	}
	content, err := readBounded(root, repoPath)
	if err != nil {
		return DiskSelection{}, false, err
	}
	return DiskSelection{Path: repoPath, Digest: Digest(content), Content: content}, true, nil
}

// cannotBeThePolicy is whether the check found nothing usable at the path (absent, a link, or not a file), as
// opposed to a path it could not judge, which is an error and not a different policy.
func cannotBeThePolicy(err error) bool {
	var refusal *ConfigError
	if !errors.As(err, &refusal) {
		return false
	}
	switch refusal.PolicyCause {
	case projectreadiness.CauseSymbolicLink, projectreadiness.CauseNotARegularFile, projectreadiness.CauseParentNotDirectory:
		return true
	}
	return refusal.AbsentFromDisk()
}
