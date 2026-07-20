// Command acceptance-guard-preflight is the activation entry point for
// internal/acceptanceharness's ambient-credential guard: run it before any
// acceptance suite starts (see mise.toml's test-acceptance-fast task). It
// scans the real process environment and default ambient-credential file
// locations and exits non-zero with a diagnostic on stderr if anything is
// found, rather than silently sanitizing and continuing.
package main

import (
	"os"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func main() {
	if !acceptanceharness.RejectAmbientCredentials(os.Stderr) {
		os.Exit(1)
	}
}
