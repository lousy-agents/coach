package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"

	"os"
)

// flagValueCheck is one flag value check; want completes the sentence
// "must be ..." in the usage error.
type flagValueCheck struct {
	flag  string
	value string
	valid bool
	want  string
}

func (c flagValueCheck) message() string {
	return fmt.Sprintf("coach: invalid --%s value %q: must be %s", c.flag, c.value, c.want)
}

// flagValueChecks is ordered: the first invalid flag is the one reported.
func flagValueChecks(f codesignalFlags) []flagValueCheck {
	return []flagValueCheck{
		{"format", f.format, f.format == "text" || f.format == "json", `"text" or "json"`},
		{"scope", f.scope, f.scope == "production" || f.scope == "all", `"production" or "all"`},
		{"min-severity", f.minSeverity, validMinSeverityFlag(f), severityFloorsWant()},
		{"top", f.top, validTopFlag(f), "a positive integer"},
		{"project-language", f.projectLanguage, f.projectLanguage == "go" || f.projectLanguage == "typescript", `"go" or "typescript"`},
	}
}

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
	for _, check := range flagValueChecks(f) {
		if !check.valid {
			return check.message()
		}
	}
	if len(positional) > 0 {
		return fmt.Sprintf("coach: unexpected positional argument %q", positional[0])
	}
	return ""
}

func renderReport(report *codesignal.Report, format string, renderOptions codesignalcli.RenderOptions, stdout, stderr *os.File) int {
	if format == "json" {
		encoded, err := codesignalcli.RenderJSON(report)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: encoding report: %s\n", err)
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprint(stdout, codesignalcli.RenderTextWithOptions(report, renderOptions)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
		return 1
	}
	return 0
}
