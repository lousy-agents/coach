package main

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

	"os"

	"strconv"
)

// runCheckProject dispatches `coach codesignal --check-project`: resolve
// HEAD, compute the read-only readiness result, and render it. A revision
// resolution or repository-inspection failure exits 1; an actionable
// readiness gap is still exit 0 -- the result IS the deliverable, and
// callers must read status/gaps, not the exit code.
func runCheckProject(dir string, f codesignalFlags, stdout, stderr *os.File) int {
	revision, err := gitrepo.ResolveBaselineRevision(dir)
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
		if suggestFlagRequestsConfig(arg) {
			return true
		}
	}
	return false
}

func suggestFlagRequestsConfig(arg string) bool {
	if arg == "--suggest-project-config" || arg == "-suggest-project-config" {
		return true
	}
	value, ok := suggestProjectConfigFlagValue(arg)
	if !ok {
		return false
	}
	requested, err := strconv.ParseBool(value)
	return err != nil || requested
}
