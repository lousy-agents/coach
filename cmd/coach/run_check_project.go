package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/render"
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

	result, err := projectcheck.Run(dir, revision, f.projectConfig)
	if err != nil {
		return classifyAnalysisError(err, f.projectLanguage, nonInteractiveRequested(f), stderr)
	}

	if f.format == "json" {
		encoded, err := render.ReadinessJSON(result)
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

	if _, err := fmt.Fprint(stdout, render.ReadinessText(result)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal --check-project: writing result: %s\n", err)
		return 1
	}
	return 0
}
