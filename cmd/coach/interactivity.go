package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
)

// nonInteractiveRequested reports whether f's own --no-interactive flag or a
// non-empty CI environment variable requests that a scan treat itself as
// non-interactive, even when a real controlling terminal is attached. CI
// systems conventionally set CI without anyone passing a flag (GitHub
// Actions, GitLab CI, Jenkins, CircleCI, ...), and a pty-allocating CI job
// still reports a genuine controlling terminal (docker run -t, ssh -t,
// script -qec) -- honoring CI here, not only the flag, is what actually
// prevents an unattended interactive offer from hanging such a job forever.
func nonInteractiveRequested(f codesignalFlags) bool {
	return f.noInteractive || os.Getenv("CI") != ""
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

func interruptibleContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
