package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

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
// *ProjectConfigErrorWithReadiness) that offerAuthoring just matched. It is printed before the interactive session opens so AC-SET-13's
// "report all gaps" clause holds on this controlling-terminal branch too, not
// only on classifyAnalysisError's no-controlling-terminal path: without it, a
// simultaneously failing compiler check would stay masked until the customer
// approved, committed, and reran, discovering the compiler gap only then.
func runScanProjectConfigAuthoring(dir string, f codesignalFlags, scanErr error, stdout, stderr *os.File, ports scanPorts) int {
	printProjectConfigGapBeforeAuthoring(scanErr, stderr)

	exitCode := authorProjectConfigTypeScript(dir, f, os.Stdin, stdout, stderr, ports)
	if exitCode != 0 {
		return exitCode
	}
	fmt.Fprintln(stderr, candidateClosingInstruction(scanErr, f))
	return 2
}

// candidateClosingInstruction says where the approved candidate goes: over the file that exists when its document
// is not valid, and at the path the user named when nothing is there yet.
func candidateClosingInstruction(scanErr error, f codesignalFlags) string {
	const written = "coach codesignal: the approved TypeScript project-config candidate was written to stdout and no file was created; "
	path := projectconfig.CanonicalPath
	if f.namedPolicy.Explicit {
		path = f.namedPolicy.Path
	}
	var configErr *projectconfig.ConfigError
	if errors.As(scanErr, &configErr) && configErr.ReplacesPolicy() {
		return fmt.Sprintf(written+"review it, save it as %q replacing the current file, commit it, then rerun this scan once it is committed.", path)
	}
	return fmt.Sprintf(written+"save it to %q at the repository root, review it, commit it, then rerun this scan once it is committed.", path)
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
		for _, line := range withReadiness.AlsoFailingLines() {
			fmt.Fprintln(stderr, line)
		}
	}
}
