package projectconfig

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const (
	refusalOversize   = "larger than the policy size limit"
	refusalUnreadable = "unreadable"
	refusalInvalid    = "not a valid project policy"
	refusalNotRegular = "not a regular file"
	refusalLink       = "a symbolic link"
	refusalParent     = "a file stands where its directory should be"
)

// DiskPolicyRefusal is why the policy on disk cannot be used. Reason is one of a fixed set of phrases, so a caller can
// print it as it stands. Directory is set where the entry that is not a regular file is a directory, which the Reason
// does not say: a named pipe has the same one.
type DiskPolicyRefusal struct {
	Path, Reason string
	Directory    bool
}

func (refusal *DiskPolicyRefusal) Error() string { return refusal.Path + ": " + refusal.Reason }

// Cause is the stable word for the refusals that have one, which are the entries that cannot be a file. A policy that is
// too large or not valid has none: its reason is the whole answer.
func (refusal DiskPolicyRefusal) Cause() projectreadiness.PolicyCause {
	return refusalCauses[refusal.Reason]
}

var refusalCauses = map[string]projectreadiness.PolicyCause{
	refusalNotRegular: projectreadiness.CauseNotARegularFile,
	refusalLink:       projectreadiness.CauseSymbolicLink,
	refusalParent:     projectreadiness.CauseParentNotDirectory,
	refusalUnreadable: projectreadiness.CauseUnreadableOnDisk,
}

// entryRefusalReasons name what is wrong with an entry, by the cause the disk check gave; a cause missing here is a
// path the process could not check, which reads as unreadable.
var entryRefusalReasons = map[projectreadiness.PolicyCause]string{
	projectreadiness.CauseNotARegularFile:    refusalNotRegular,
	projectreadiness.CauseSymbolicLink:       refusalLink,
	projectreadiness.CauseParentNotDirectory: refusalParent,
}

// invalidDiskPolicyError marks a Parse failure, so ClassifyDiskPolicyError can tell a policy that was read but is
// wrong from a read that failed: both are plain errors otherwise.
type invalidDiskPolicyError struct{ cause error }

func (err *invalidDiskPolicyError) Error() string {
	return fmt.Sprintf("invalid project policy: %s", err.cause)
}

func (err *invalidDiskPolicyError) Unwrap() error { return err.cause }

// UsableDiskPolicy is the policy in the bytes SelectOnDisk read, judged by the same schema Parse applies to a committed
// policy. It never looks at the file again, so what it accepts is what the selection's digest names.
func UsableDiskPolicy(selection DiskSelection) (Config, error) {
	config, err := Parse(selection.Content)
	if err != nil {
		return Config{}, &invalidDiskPolicyError{cause: err}
	}
	return config, nil
}

// ClassifyDiskPolicyError is the refusal for an error from SelectOnDisk or UsableDiskPolicy, and nil for no error and
// for an error that is not about the file's own state (git failing to find the repository, a timeout), which stays the
// operational failure it is: reading it as an unusable policy would send a broken environment to the user as a defect
// of their file.
func ClassifyDiskPolicyError(path string, err error) *DiskPolicyRefusal {
	var typed *DiskPolicyRefusal
	var oversize *oversizePolicyError
	var invalid *invalidDiskPolicyError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &typed):
		return typed
	case errors.As(err, &oversize):
		return &DiskPolicyRefusal{Path: path, Reason: refusalOversize}
	case errors.As(err, &invalid):
		return &DiskPolicyRefusal{Path: path, Reason: refusalInvalid}
	}
	return nil
}
