package codesignalcli

import (
	"bufio"

	"fmt"
	"io"

	"github.com/lousy-agents/coach/pkg/projectmodel"

	"strings"
)

func promptForRequiredLayer(out io.Writer, reader *bufio.Reader, layers []projectConfigLayer) (requiredLayer string, cancelled bool) {
	for {
		fmt.Fprintln(out, "Enter the name of a required intermediary layer, or leave blank for none:")
		fmt.Fprint(out, "> ")
		answer, _ := readLine(reader)
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return "", false
		}
		known := layerNameDeclared(answer, layers)
		if !known && promptRetryOrCancel(out, reader, fmt.Sprintf("required_layer references undefined layer %q", answer)) {
			return "", true
		}
		if !known {
			continue
		}

		return answer, false
	}
}

// promptForRoots prints discovered.Roots/Candidates as a suggestion, then
// repeatedly reads one line of the user's own answer until it resolves to a
// non-empty set of valid repository-relative directories -- the same
// explain-and-retry-or-cancel treatment every other field (layers, forbidden
// pairs, required layer) already gets. There is no answer that selects zero
// roots: validateProjectConfigRoots (the same schema validator the write
// stage applies) never accepts an empty root list, so accepting one here
// would only defer a certain failure to the very end of the session, after
// every remaining stage and the approval gate had already been answered. It
// never returns discovered.Roots itself when the user selects nothing: an
// accepted selection is always something the user typed or picked.
func promptForRoots(out io.Writer, reader *bufio.Reader, discovered projectmodel.TSRootDiscoveryResult) (roots []string, cancelled bool) {
	printRootSuggestions(out, discovered)

	for {
		fmt.Fprintln(out, "Select the roots to include: enter comma-separated numbers from the list above and/or directory paths, then press Enter. At least one repository-relative root is required.")
		fmt.Fprint(out, "> ")

		answer, _ := readLine(reader)
		selected, parseErr := parseRootSelection(answer, discovered.Roots)
		if parseErr == nil {
			parseErr = validateRootSelection(selected)
		}
		if parseErr != nil && promptRetryOrCancel(out, reader, parseErr.Error()) {
			return nil, true
		}
		if parseErr != nil {
			continue
		}

		return selected, false
	}
}
func layerNameDeclared(name string, layers []projectConfigLayer) bool {
	for _, layer := range layers {
		if layer.Name == name {
			return true
		}
	}
	return false
}
