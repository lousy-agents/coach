package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// UnusableEntryOnDisk is the refusal for a path the working tree holds an entry at that cannot be the policy: a
// directory, a link, a pipe, or a path below a file. It is nil for a path with nothing there and for a regular file.
func UnusableEntryOnDisk(dir, repoPath string) *DiskPolicyRefusal {
	rootDir, err := gitrepo.RepositoryRootWith(NewGitRunner(), dir)
	if err != nil {
		return ClassifyDiskPolicyError(repoPath, err)
	}
	err = checkRegularFileUnder(rootDir, repoPath)
	if err == nil {
		return nil
	}
	var verdict *ConfigError
	if !errors.As(err, &verdict) {
		return ClassifyDiskPolicyError(repoPath, err)
	}
	if verdict.AbsentFromDisk() {
		return nil
	}
	reason, known := entryRefusalReasons[verdict.PolicyCause]
	if !known {
		reason = refusalUnreadable
	}
	return &DiskPolicyRefusal{Path: repoPath, Reason: reason}
}

// entryRefusalReasons name what is wrong with an entry, by the cause the disk check gave; a cause missing here is a
// path the process could not check, which reads as unreadable.
var entryRefusalReasons = map[projectreadiness.PolicyCause]string{
	projectreadiness.CauseNotARegularFile:    refusalNotRegular,
	projectreadiness.CauseSymbolicLink:       refusalLink,
	projectreadiness.CauseParentNotDirectory: refusalParent,
}
