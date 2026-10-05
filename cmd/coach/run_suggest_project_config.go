package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/configauthoring"
)

func runSuggestProjectConfig(dir string, f codesignalFlags, stdout, stderr *os.File) int {
	result := configauthoring.Suggest(dir, f.output, f.outputSet)
	if len(result.Envelope) > 0 {
		if _, writeErr := stderr.Write(result.Envelope); writeErr != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing diagnostic: %s\n", writeErr)
			return 1
		}
	}
	if result.ExitCode == 0 && len(result.Candidate) > 0 {
		if _, writeErr := stdout.Write(result.Candidate); writeErr != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing candidate: %s\n", writeErr)
			return 1
		}
	}
	return result.ExitCode
}
