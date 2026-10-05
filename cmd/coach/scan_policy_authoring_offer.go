package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

// scanShouldAuthorProjectConfig reports whether a real scan's (not
// --check-project/--suggest-project-config/--prepare-compiler) analysis
// error is AC-POL-8's guided-authoring case: a TypeScript policy that was
// never committed at all (*projectconfig.ConfigError with Kind ==
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
// nonInteractiveRequested in interactivity.go) all fall through to
// classifyAnalysisError's existing message-only path unchanged.
func scanShouldAuthorProjectConfig(err error, language, configPath string, noInteractive bool) bool {
	if language != "typescript" {
		return false
	}
	if projectconfig.ValidatePath(configPath) != nil {
		return false
	}
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) {
		return false
	}
	if configErr.Kind != projectconfig.KindNotFound {
		return false
	}
	if noInteractive {
		return false
	}
	return terminal.HasControllingTerminal(os.Stdin)
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

// printProjectConfigGapBeforeAuthoring prints the same two lines
// classifyAnalysisError would have printed for scanErr's class-2 config
// failure -- its own message, plus AC-SET-13's masked-gap lines when scanErr
// was wrapped with a readiness snapshot -- before guided authoring takes over
// stderr with its own prompts.
func printProjectConfigGapBeforeAuthoring(scanErr error, stderr *os.File) {
	var configErr *projectconfig.ConfigError
	if !errors.As(scanErr, &configErr) {
		return
	}
	fmt.Fprintln(stderr, configErr.Message)

	var withReadiness *tssetup.ProjectConfigErrorWithReadiness
	if errors.As(scanErr, &withReadiness) {
		for _, line := range tssetup.AlsoFailingGapLines(withReadiness.Readiness, withReadiness.ConfigPath) {
			fmt.Fprintln(stderr, line)
		}
	}
}
