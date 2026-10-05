package configauthoring

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func printRootSuggestions(out io.Writer, discovered projectmodel.TSRootDiscoveryResult) {
	if len(discovered.Roots) > 0 {
		fmt.Fprintln(out, "Discovered TypeScript roots (directories with a tsconfig.json):")
		for i, root := range discovered.Roots {
			fmt.Fprintf(out, "  %d. %s\n", i+1, root)
		}
	} else {
		fmt.Fprintln(out, "No TypeScript roots (tsconfig.json) were discovered.")
	}
	if len(discovered.Candidates) > 0 {
		fmt.Fprintln(out, "Other directories with a package.json but no tsconfig.json of their own:")
		for _, candidate := range discovered.Candidates {
			fmt.Fprintf(out, "  - %s\n", candidate)
		}
	}
}

func validateRootSelection(selected []string) error {
	if len(selected) == 0 {
		return fmt.Errorf("at least one repository-relative root must be selected")
	}
	if len(selected) > projectconfig.MaxRoots {
		return fmt.Errorf("roots exceed budget of %d entries", projectconfig.MaxRoots)
	}
	for _, root := range selected {
		if err := projectconfig.ValidateDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}
	return nil
}

// promptRetryOrCancel explains why the user's last answer was rejected and
// asks whether to retry that same field or cancel the whole authoring
// session. It returns true when the user's reply is "cancel"
// (case-insensitive), or when the input is exhausted (EOF): an exhausted
// reader can never supply a different answer on a later retry, so treating
// it as anything but cancellation would spin the caller's prompt loop
// forever. Any other reply -- including "retry" -- is treated as a request
// to retry, so the caller's own prompt loop asks the field again.
func promptRetryOrCancel(out io.Writer, reader *bufio.Reader, explanation string) bool {
	fmt.Fprintf(out, "That answer is invalid: %s\n", explanation)
	fmt.Fprintln(out, "Type 'retry' to try again, or 'cancel' to cancel authoring:")
	fmt.Fprint(out, "> ")
	reply, unreadable := prompt.ReadLine(reader)
	if unreadable {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(reply), "cancel")
}
