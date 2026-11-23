package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// selectionFailurePath names the file a failed selection stopped at. SelectOnDisk reports the error and not the path,
// and it stops at the first candidate that is a regular file it cannot read, so asking for the canonical file alone
// tells whether that was the one.
func selectionFailurePath(dir, namedPath string) string {
	if namedPath != "" {
		return namedPath
	}
	if _, err := projectconfig.SelectOnDisk(dir, projectconfig.CanonicalPath); err != nil {
		return projectconfig.CanonicalPath
	}
	return projectconfig.DefaultPath
}

// judgmentOfRefusal is the policy_invalid gap for a regular file that cannot be used. Authoring refuses a path that
// exists, so the gap offers no way out: a file over the size limit or with content that is no policy carries the cause
// undetermined, and one the process could not read carries unreadable_on_disk.
func judgmentOfRefusal(path string, cause error, policy policyref.Ref) (policyJudgment, error) {
	refusal := projectconfig.ClassifyDiskPolicyError(path, cause).ExplicitError()
	check, _, err := policyGapFrom(refusal)
	return policyJudgment{path: path, check: wordedForThePolicy(check, policy, path)}, err
}
