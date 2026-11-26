package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// ExplicitError is the refusal of a policy the user named with --project-config: that path is the policy or the scan
// stops, never a different policy or none.
func (refusal *DiskPolicyRefusal) ExplicitError() error {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: refusal.Cause(), Message: fmt.Sprintf("coach codesignal: --project-config %q cannot be used on disk (project_config_invalid): %s", refusal.Path, refusal.Words())}
}

// RefusalForError is ClassifyDiskPolicyError for a caller that has no operational failure to separate: an error that is
// not about the file's own state reads as a file that cannot be read. The project checks report that as a gap, where a
// scan stops with the failure.
func RefusalForError(path string, err error) *DiskPolicyRefusal {
	if refusal := ClassifyDiskPolicyError(path, err); refusal != nil || err == nil {
		return refusal
	}
	return &DiskPolicyRefusal{Path: path, Reason: refusalUnreadable}
}
