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
// print it as it stands.
type DiskPolicyRefusal struct{ Path, Reason string }

func (refusal *DiskPolicyRefusal) Error() string { return refusal.Path + ": " + refusal.Reason }

// ExplicitError is the refusal of a policy the user named with --project-config: that path is the policy or the scan
// stops, never a different policy or none.
func (refusal *DiskPolicyRefusal) ExplicitError() error {
	cause := projectreadiness.PolicyCause("")
	if refusal.Reason == refusalUnreadable {
		cause = projectreadiness.CauseUnreadableOnDisk
	}
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: cause, Message: fmt.Sprintf("coach codesignal: --project-config %q cannot be used on disk (project_config_invalid): %s", refusal.Path, refusal.Reason)}
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

// ClassifyDiskPolicyError is the refusal for an error from SelectOnDisk or UsableDiskPolicy, or nil for no error.
func ClassifyDiskPolicyError(path string, err error) *DiskPolicyRefusal {
	if err == nil {
		return nil
	}
	var oversize *oversizePolicyError
	var invalid *invalidDiskPolicyError
	switch {
	case errors.As(err, &oversize):
		return &DiskPolicyRefusal{Path: path, Reason: refusalOversize}
	case errors.As(err, &invalid):
		return &DiskPolicyRefusal{Path: path, Reason: refusalInvalid}
	}
	return &DiskPolicyRefusal{Path: path, Reason: refusalUnreadable}
}
