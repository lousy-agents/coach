package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

const prepareCompilerMiseUsagePrefix = "coach codesignal --baseline --prepare-compiler --project-language typescript"

// runPrepareCompilerMiseTypeScript dispatches `coach codesignal --baseline
// --prepare-compiler --project-language typescript`'s consented mise
// compiler-setup flow. The interactivity check on stdin runs before any
// revision resolution or readiness computation: with no controlling terminal
// -- or with one nobody is attending, which nonInteractiveRequested is what
// detects -- this never prompts and never mutates mise state.
func runPrepareCompilerMiseTypeScript(dir string, f codesignalFlags, stdin, stdout, stderr *os.File) int {
	if reason := interactiveRefusalReason(f, stdin); reason != "" {
		fmt.Fprintf(stderr, "%s: %s; refusing to enter interactive compiler setup or mutate mise state.\n", prepareCompilerMiseUsagePrefix, reason)
		return 2
	}
	return prepareCompilerMiseTypeScript(dir, stdin, stdout, stderr, f.projectConfig)
}

// prepareCompilerMiseTypeScript writes its entire interactive transcript
// (choice list, preview, and every prompt) to stderr, never stdout: this
// flow never produces a report of its own. stdout is accepted as a
// parameter only for signature symmetry with the rest of this package's CLI
// dispatch functions and is otherwise unused.
func prepareCompilerMiseTypeScript(dir string, stdin, stdout, stderr *os.File, projectConfigPath string) int {
	revision, err := gitrepo.ResolveBaselineRevision(dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not resolve the baseline revision: %s\n", prepareCompilerMiseUsagePrefix, err)
		return 3
	}

	readiness, err := projectcheck.Run(dir, revision, projectConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not compute project readiness: %s\n", prepareCompilerMiseUsagePrefix, err)
		return 3
	}

	ctx, stop := interruptibleContext()
	defer stop()
	result := tssetup.RunPrepareCompilerMiseSetup(ctx, dir, revision, projectConfigPath, readiness, stdin, stderr)
	return reportPrepareCompilerMiseResult(result, stderr)
}
