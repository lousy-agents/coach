package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/configauthoring"
)

// reportAuthoringResult translates one AuthorProjectConfig session outcome
// into the process's exit code. A declined/cancelled session and every
// failure mode share exit 2, the same usage/discovery-failure exit code
// --suggest-project-config already uses; only Approved with no validation,
// existing-target, or write error is success. The approved candidate itself
// has already reached stdout or disk inside AuthorProjectConfig -- there is
// nothing left to write here.
func reportAuthoringResult(result configauthoring.Result, stderr *os.File) int {
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
