package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// invalidProjectConfigMessage is the usage error for a --project-config value
// that cannot name a policy, and "" when the path is usable.
func invalidProjectConfigMessage(path string) string {
	if err := projectconfig.ValidatePath(path); err != nil {
		return fmt.Sprintf("coach: --project-config %q is invalid: %s", path, err)
	}
	return ""
}
