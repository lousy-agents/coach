package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"

	"os"
)

func validateCodesignalFlags(f codesignalFlags) string {
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
	return ""
}
func renderReport(report *codesignal.Report, format string, stdout, stderr *os.File) int {
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

	if _, err := fmt.Fprint(stdout, codesignalcli.RenderText(report)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
		return 1
	}
	return 0
}
