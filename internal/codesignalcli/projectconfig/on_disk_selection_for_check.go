package projectconfig

import (
	"errors"
	"os"
)

// SelectOnDiskForCheck is SelectOnDisk for the project checks, which judge an entry that cannot be the policy from the
// working tree themselves, along with what the index and the revision hold for it. A link, a directory, a pipe, or a path
// below a file selects nothing and the next candidate is tried; a regular file that cannot be read in full within MaxBytes
// is still a refusal.
func SelectOnDiskForCheck(dir, namedPath string) (DiskSelection, error) {
	return selectCandidates(NewGitRunner(), dir, namedPath, refusalOfEntryForCheck)
}

func refusalOfEntryForCheck(root *os.Root, repoPath string, err error) error {
	refusal := refusalOfEntry(root, repoPath, err)
	var typed *DiskPolicyRefusal
	if errors.As(refusal, &typed) && typed.Reason != refusalUnreadable {
		return nil
	}
	return refusal
}
