package tssetup

import (
	"fmt"
	"io"
	"strconv"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var policyDisclosures = map[string]string{
	projectreadiness.WarnPolicyNotCommitted:    "the policy on disk is not committed at the analyzed revision",
	projectreadiness.WarnPolicyDiffersFromHead: "the policy on disk differs from the analyzed revision's copy",
	projectreadiness.WarnPolicyHeadUnreadable:  "the analyzed revision's copy of the policy on disk could not be read, so the two cannot be compared",
}

// printPolicyDisclosure says, before any prompt, how the policy this setup is read from relates to the analyzed revision.
// The policy on disk is what the scan uses, so the disclosure informs the choice and asks nothing of it.
func printPolicyDisclosure(out io.Writer, readiness *projectreadiness.Result) {
	if readiness == nil {
		return
	}
	for _, warning := range readiness.Warnings {
		why, disclosed := policyDisclosures[warning.Code]
		if !disclosed {
			continue
		}
		fmt.Fprintf(out, "TypeScript compiler setup: warning %s (root=%s): %s.\n", warning.Code, printable(warning.Root), why)
	}
}

// printable is text from the repository made safe to write to a terminal: text Go would quote unchanged is returned
// as it is, and anything else is shown in Go syntax, so no control character reaches the terminal.
func printable(text string) string {
	quoted := strconv.Quote(text)
	if quoted == `"`+text+`"` {
		return text
	}
	return quoted
}
