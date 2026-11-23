package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// UnusableEntryOnDisk is the refusal for a path the working tree holds an entry at that cannot be the policy: a
// directory, a link, a pipe, or a path below a file. It is nil for a path with nothing there and for a regular file.
func UnusableEntryOnDisk(dir, repoPath string) *DiskPolicyRefusal {
	rootDir, err := gitrepo.RepositoryRootWith(NewGitRunner(), dir)
	if err != nil {
		return RefusalForError(repoPath, err)
	}
	err = checkRegularFileUnder(rootDir, repoPath)
	if err == nil {
		return nil
	}
	var verdict *ConfigError
	if !errors.As(err, &verdict) {
		return RefusalForError(repoPath, err)
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
