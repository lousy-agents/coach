package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func validateCheckProjectFlags(f codesignalFlags, setFlags map[string]bool, positional []string, checkProjectCount int) string {
	if checkProjectCount > 1 {
		return "coach: --check-project may only be provided once"
	}
	if !f.baseline {
		return "coach: --check-project requires --baseline"
	}
	if f.projectLanguage != "typescript" {
		return fmt.Sprintf("coach: --check-project requires --project-language typescript (got %q)", f.projectLanguage)
	}
	if f.format != "text" && f.format != "json" {
		return fmt.Sprintf("coach: invalid --format value %q: must be \"text\" or \"json\"", f.format)
	}
	if f.projectConfigSet {
		if err := projectconfig.ValidatePath(f.projectConfig); err != nil {
			return fmt.Sprintf("coach: --project-config %q is invalid: %s", f.projectConfig, err)
		}
	}
	allowedWithCheckProject := map[string]bool{"check-project": true, "baseline": true, "project-language": true, "project-config": true, "format": true}
	if name, disallowed := firstDisallowedFlag(setFlags, allowedWithCheckProject); disallowed {
		return fmt.Sprintf("coach: --check-project cannot be combined with --%s", name)
	}
	return rejectPositionalArgs("check-project", positional, "")
}
