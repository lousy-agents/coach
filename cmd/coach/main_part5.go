package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"

	"os"

	"strconv"
)

// runCheckProject dispatches `coach codesignal --check-project`: resolve
// HEAD, compute the read-only readiness result, and render it. A revision
// resolution or repository-inspection failure exits 1; an actionable
// readiness gap is still exit 0 -- the result IS the deliverable, and
// callers must read status/gaps, not the exit code.
func runCheckProject(dir string, f codesignalFlags, stdout, stderr *os.File) int {
	revision, err := codesignalcli.ResolveBaselineRevision(dir)
	if err != nil {
		return classifyAnalysisError(err, f.projectLanguage, nonInteractiveRequested(f), stderr)
	}

	result, err := codesignalcli.CheckProjectReadiness(dir, revision, f.projectConfig)
	if err != nil {
		return classifyAnalysisError(err, f.projectLanguage, nonInteractiveRequested(f), stderr)
	}

	if f.format == "json" {
		encoded, err := codesignalcli.RenderReadinessJSON(result)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal --check-project: encoding result: %s\n", err)
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			fmt.Fprintf(stderr, "coach codesignal --check-project: writing result: %s\n", err)
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprint(stdout, codesignalcli.RenderReadinessText(result)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal --check-project: writing result: %s\n", err)
		return 1
	}
	return 0
}
func suggestProjectConfigRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--suggest-project-config" || arg == "-suggest-project-config" {
			return true
		}
		if value, ok := suggestProjectConfigFlagValue(arg); ok {
			if requested, err := strconv.ParseBool(value); err == nil && !requested {
				continue
			}
			return true
		}
	}
	return false
}
