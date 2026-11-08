package main

import (
	"fmt"
)

// validatePrepareCompilerFlags omits --format from its allowlist: this flow
// never renders a report, so there is no format to choose.
func validatePrepareCompilerFlags(f codesignalFlags, setFlags map[string]bool, positional []string, prepareCompilerCount int) string {
	if prepareCompilerCount > 1 {
		return "coach: --prepare-compiler may only be provided once"
	}
	if !f.baseline {
		return "coach: --prepare-compiler requires --baseline"
	}
	if f.projectLanguage != "typescript" {
		return fmt.Sprintf("coach: --prepare-compiler requires --project-language typescript (got %q)", f.projectLanguage)
	}
	if message := invalidNamedPolicyMessage(f.namedPolicy); message != "" {
		return message
	}
	allowedWithPrepareCompiler := map[string]bool{"prepare-compiler": true, "baseline": true, "project-language": true, "project-config": true, "no-interactive": true}
	if name, disallowed := firstDisallowedFlag(setFlags, allowedWithPrepareCompiler); disallowed {
		return fmt.Sprintf("coach: --prepare-compiler cannot be combined with --%s", name)
	}
	return rejectPositionalArgs("prepare-compiler", positional, "")
}
