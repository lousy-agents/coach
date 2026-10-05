package main

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"

	"os"

	"strings"
)

// scanShouldOfferCompilerSetup reports whether err is a scan-time
// CompilerUnresolvedError carrying its own readiness snapshot (wrapped by
// runBaselineAnalysis/runDiffAnalysis for AC-SET-9's controlling-terminal
// branch), noInteractive is false, and a controlling terminal is available
// to run the interactive offer on. With noInteractive true or no controlling
// terminal, classifyAnalysisError's existing message-only path (unwrapping
// straight through to the plain *CompilerUnresolvedError) is unchanged --
// this is what keeps a pty-allocating but genuinely unattended invocation
// (R1) from opening a prompt nobody will ever answer.
func scanShouldOfferCompilerSetup(err error, noInteractive bool) (*tssetup.CompilerUnresolvedErrorWithReadiness, bool) {
	var wrapped *tssetup.CompilerUnresolvedErrorWithReadiness
	if !errors.As(err, &wrapped) {
		return nil, false
	}
	if noInteractive || !terminal.HasControllingTerminal(os.Stdin) {
		return nil, false
	}
	return wrapped, true
}

// interactiveRefusalReason names why a command that can only work by
// prompting must refuse, or "" when it may proceed. The two halves are
// reported apart because they are not the same situation: a piped invocation
// has nobody to ask, while --no-interactive or a non-empty CI means a real
// terminal is attached and Coach was asked not to use it.
//
// The absent terminal is reported first when both hold, because it is the
// fact the customer can do something about: on a CI runner CI is set for
// every job, so telling a genuinely piped invocation that it was declined
// would point at an environment variable whose removal changes nothing --
// there is still no terminal to prompt on.
func interactiveRefusalReason(f codesignalFlags, stdin *os.File) string {
	if !terminal.HasControllingTerminal(stdin) {
		return "no controlling terminal is available"
	}
	if nonInteractiveRequested(f) {
		return "this invocation is non-interactive (--no-interactive, or a non-empty CI environment variable)"
	}
	return ""
}

// withheldSetupChoicesLine renders AvailableSetupChoices' own reasons for
// ruling out every candidate, for the one outcome that leaves the customer
// without a next step: a real compiler gap where nothing at all could be
// offered. The gap line printed above it names --check-project, which only
// re-reports the same gap, so without the reasons (an unconfigured mise, a
// manifest that declares no supported compiler, a rejected package manager)
// there is nothing to act on. It returns "" when no menu was built, so the
// runtime-boundary path's output is unchanged.
func withheldSetupChoicesLine(withheld []tssetup.WithheldChoice) string {
	if len(withheld) == 0 {
		return ""
	}
	reasons := make([]string, 0, len(withheld))
	for _, entry := range withheld {
		reasons = append(reasons, fmt.Sprintf("%s (%s)", entry.Kind, entry.Reason))
	}
	return "coach codesignal: no compiler-setup choice is executable here: " + strings.Join(reasons, ", ") + "."
}
func (b scanOfferBudget) without(kind scanOfferKind) scanOfferBudget {
	next := make(scanOfferBudget, len(b))
	for k, v := range b {
		next[k] = v
	}
	next[kind] = false
	return next
}

// classTwoReport assembles the shape every class-2 diagnostic shares (owner
// decision D3): the gap's own message, then any further gaps AC-SET-13
// requires reporting alongside it, then AC-SET-9's appended remediation when
// one is offered. An unoffered remediation is dropped rather than printed as
// a blank line.
func classTwoReport(message string, alsoFailing []string, appended string) analysisErrorReport {
	lines := append([]string{message}, alsoFailing...)
	if appended != "" {
		lines = append(lines, appended)
	}
	return analysisErrorReport{lines: lines, exitCode: 2}
}
