package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// emptyNamedPolicyMessage is the usage error for a --project-config passed with
// an empty value, and "" for any other value, including a policy that was not
// named. A scan leaves every other malformed path to the load that judges the
// policy, so only the one value that names no policy at all is refused here.
func emptyNamedPolicyMessage(policy policyref.Ref) string {
	if path, named := policy.NamedPath(); !named || path != "" {
		return ""
	}
	return invalidNamedPolicyMessage(policy)
}

// invalidNamedPolicyMessage is the usage error for a --project-config value that
// cannot name a policy, and "" when no policy was named or its path is usable.
func invalidNamedPolicyMessage(policy policyref.Ref) string {
	path, named := policy.NamedPath()
	if !named {
		return ""
	}
	if err := projectconfig.ValidatePath(path); err != nil {
		return fmt.Sprintf("coach: --project-config %q is invalid: %s", path, err)
	}
	return ""
}
