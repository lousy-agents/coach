package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"

	"os"

	"strconv"
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
		if err := codesignalcli.ValidateProjectConfigPath(f.projectConfig); err != nil {
			return fmt.Sprintf("coach: --project-config %q is invalid: %s", f.projectConfig, err)
		}
	}
	allowedWithCheckProject := map[string]bool{"check-project": true, "baseline": true, "project-language": true, "project-config": true, "format": true}
	if name, disallowed := firstDisallowedFlag(setFlags, allowedWithCheckProject); disallowed {
		return fmt.Sprintf("coach: --check-project cannot be combined with --%s", name)
	}
	return rejectPositionalArgs("check-project", positional, "")
}
func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, topLevelUsage)
		return 2
	}

	switch args[0] {
	case "--help", "-h":
		fmt.Fprintln(stdout, topLevelUsage)
		return 0
	case "--version":
		fmt.Fprintln(stdout, version)
		return 0
	case "codesignal":
		return runCodesignal(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s\ncoach: unknown command %q\n", topLevelUsage, args[0])
		return 2
	}
}
func (c *countingBoolFlag) String() string {
	if c == nil {
		return "false"
	}
	return strconv.FormatBool(c.value)
}
