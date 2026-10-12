package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func wrapScanAnalysisError(err error, dir, revision, configPath string, stderr *os.File) error {
	var unresolved *tstoolchain.CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return tssetup.WrapCompilerUnresolvedErrorWithReadiness(unresolved, dir, revision, configPath)
	}
	var runtimeErr *tstoolchain.RuntimeUnresolvedError
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
	return nil
}

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
