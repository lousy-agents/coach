package projectcheck

import "github.com/lousy-agents/coach/internal/codesignalcli/policyref"

// JudgedPolicyPath is the path whose policy readiness judges for policy: the path the user named, or the one discovery
// selects at revision, which is the default path when it selects none. A refusal that would end the check is no path,
// and is returned.
func JudgedPolicyPath(dir, revision string, policy policyref.Ref) (string, error) {
	path, _, err := resolveCheckPolicy(dir, revision, policy)
	return path, err
}
