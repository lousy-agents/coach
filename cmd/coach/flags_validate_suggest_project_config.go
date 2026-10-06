package main

import (
	"fmt"
)

func validateSuggestProjectConfigFlags(f codesignalFlags, setFlags map[string]bool, positional []string, suggestCount, outputCount int) string {
	const suffix = " (project_config_suggestion_invalid_arguments)"
	if suggestCount > 1 {
		return "coach: --suggest-project-config may only be provided once" + suffix
	}
	if outputCount > 1 {
		return "coach: --output may only be provided once" + suffix
	}
	if !f.baseline {
		return "coach: --suggest-project-config requires --baseline" + suffix
	}
	allowedWithSuggest := map[string]bool{"suggest-project-config": true, "output": true, "baseline": true}
	if f.projectLanguage == "typescript" {
		// --no-interactive rides with the language: only the TypeScript
		// path prompts, so accepting it for a Go candidate would advertise
		// a guard over a flow that never opens a session.
		allowedWithSuggest["project-language"] = true
		allowedWithSuggest["no-interactive"] = true
	}
	if name, disallowed := firstDisallowedFlag(setFlags, allowedWithSuggest); disallowed {
		return fmt.Sprintf("coach: --suggest-project-config cannot be combined with --%s%s", name, suffix)
	}
	return rejectPositionalArgs("suggest-project-config", positional, suffix)
}
