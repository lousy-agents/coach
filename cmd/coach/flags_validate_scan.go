package main

import (
	"fmt"
)

func validateCodesignalFlags(f codesignalFlags, positional []string) string {
	if f.outputSet {
		return "coach: --output requires --suggest-project-config"
	}
	if f.baseline && f.base != "" {
		return "coach: --baseline and --base are mutually exclusive: choose a Repository Baseline scan (--baseline) or a diff comparison (--base), not both"
	}
	if !f.baseline && f.base == "" {
		return "coach: missing required --base flag"
	}
	if f.format != "text" && f.format != "json" {
		return fmt.Sprintf("coach: invalid --format value %q: must be \"text\" or \"json\"", f.format)
	}
	if f.scope != "production" && f.scope != "all" {
		return fmt.Sprintf("coach: invalid --scope value %q: must be \"production\" or \"all\"", f.scope)
	}
	if f.projectLanguage != "go" && f.projectLanguage != "typescript" {
		return fmt.Sprintf("coach: invalid --project-language value %q: must be \"go\" or \"typescript\"", f.projectLanguage)
	}
	if len(positional) > 0 {
		return fmt.Sprintf("coach: unexpected positional argument %q", positional[0])
	}
	return ""
}
