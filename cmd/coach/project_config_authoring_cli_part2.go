package main

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"

	"os"
)

// scanShouldAuthorProjectConfig reports whether a real scan's (not
// --check-project/--suggest-project-config/--prepare-compiler) analysis
// error is AC-POL-8's guided-authoring case: a TypeScript policy that was
// never committed at all (*codesignalcli.ProjectConfigError with Kind ==
// ProjectConfigNotFound, whether or not it has been wrapped in a
// *ProjectConfigErrorWithReadiness for AC-SET-13) with a controlling
// terminal available on stdin. An unusable configPath is rejected before
// even inspecting err, since ValidateProjectConfigPath's own failure is a
// usage error, not an absent policy. Every other *ProjectConfigError kind
// (a usage error that reached classifyAnalysisError some other way, a file
// that exists uncommitted in the worktree, or a transient git operational
// failure), a CompilerUnresolvedError (--prepare-compiler's compiler-setup
// flow owns that gap instead), any other error, a non-TypeScript
// --project-language (the guided session below is TypeScript-specific), or
// no controlling terminal, or noInteractive being true (R1's escape hatch:
// --no-interactive or a non-empty CI environment variable, see
// nonInteractiveRequested in main.go) all fall through to
// classifyAnalysisError's existing message-only path unchanged.
func scanShouldAuthorProjectConfig(err error, language, configPath string, noInteractive bool) bool {
	if language != "typescript" {
		return false
	}
	if codesignalcli.ValidateProjectConfigPath(configPath) != nil {
		return false
	}
	var configErr *codesignalcli.ProjectConfigError
	if !errors.As(err, &configErr) {
		return false
	}
	if configErr.Kind != codesignalcli.ProjectConfigNotFound {
		return false
	}
	if noInteractive {
		return false
	}
	return terminal.HasControllingTerminal(os.Stdin)
}

// reportAuthoringResult translates one AuthorProjectConfig session outcome
// into the process's exit code. A declined/cancelled session and every
// failure mode share exit 2, the same usage/discovery-failure exit code
// --suggest-project-config already uses; only Approved with no validation,
// existing-target, or write error is success. The approved candidate itself
// has already reached stdout or disk inside AuthorProjectConfig -- there is
// nothing left to write here.
func reportAuthoringResult(result codesignalcli.AuthoringResult, stderr *os.File) int {
	if !result.Approved {
		fmt.Fprintf(stderr, "%s: authoring was cancelled or not approved; no policy config was written\n", authorTSUsagePrefix)
		return 2
	}
	if result.ValidationError != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, result.ValidationError)
		return 2
	}
	if result.OutputExists {
		fmt.Fprintf(stderr, "%s: --output target already exists\n", authorTSUsagePrefix)
		return 2
	}
	if result.WriteError != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, result.WriteError)
		return 2
	}
	return 0
}

// runScanProjectConfigAuthoring implements AC-POL-8. It reuses the exact
// guided authoring session `--suggest-project-config --project-language
// typescript` runs (authorProjectConfigTypeScript) rather than re-deriving
// authoring logic, but overrides that command's own success exit code: a
// successful, approved session there returns 0 because the standalone
// command's job is done once the candidate is written, whereas here the
// same candidate has not been reviewed or committed yet, so this
// invocation must still refuse to analyze it -- exit 2, no CodeSignal
// report (none is ever rendered on this path), and an instruction to
// review, commit, and rerun.
//
// A scan can never set --output (validateCodesignalFlags rejects it outside
// --suggest-project-config), so the approved candidate is emitted to stdout
// rather than written to disk -- the suggestion-mode shape the epic freezes
// for an omitted --output, and AC-EVD-8's "policy-candidate document" case.
// Nothing is created on disk here, which is why the closing instruction
// names stdout and the --project-config path the customer must save it to:
// telling them a candidate "was created" would name an artifact they cannot
// find, review, or commit.
//
// scanErr is the *ProjectConfigError (possibly wrapped in a
// *ProjectConfigErrorWithReadiness) that scanShouldAuthorProjectConfig just
// matched. It is printed before the interactive session opens so AC-SET-13's
// "report all gaps" clause holds on this controlling-terminal branch too, not
// only on classifyAnalysisError's no-controlling-terminal path: without it, a
// simultaneously failing compiler check would stay masked until the customer
// approved, committed, and reran, discovering the compiler gap only then.
func runScanProjectConfigAuthoring(dir string, f codesignalFlags, scanErr error, stdout, stderr *os.File) int {
	printProjectConfigGapBeforeAuthoring(scanErr, stderr)

	exitCode := authorProjectConfigTypeScript(dir, f, os.Stdin, stdout, stderr)
	if exitCode != 0 {
		return exitCode
	}
	fmt.Fprintf(stderr, "coach codesignal: the approved TypeScript project-config candidate was written to stdout and no file was created; save it to %q, review it, commit it, then rerun this scan once it is committed.\n", f.projectConfig)
	return 2
}
