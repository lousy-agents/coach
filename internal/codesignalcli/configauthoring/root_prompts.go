package configauthoring

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// parseRootSelection turns one line of user input into an ordered,
// deduplicated root list. A token that parses as a 1-based index into
// discoveredRoots resolves to that root; any other non-empty token is taken
// as a literal path exactly as typed. An out-of-range numeric token is
// rejected with an explanatory error rather than silently dropped: dropping
// it while keeping the rest of a valid answer would leave the resolved
// selection non-empty and so pass validateRootSelection unnoticed, meaning
// the customer's typo (or a stale suggestion list) silently selects fewer
// roots than they asked for with no indication anything was wrong.
func parseRootSelection(answer string, discoveredRoots []string) ([]string, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return nil, nil
	}

	var selected []string
	seen := map[string]bool{}
	for _, token := range splitTrimmedNonEmpty(answer, ",") {
		root, err := resolveRootToken(token, discoveredRoots)
		if err != nil {
			return nil, err
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		selected = append(selected, root)
	}
	return selected, nil
}

func resolveRootToken(token string, discoveredRoots []string) (string, error) {
	idx, err := strconv.Atoi(token)
	if err != nil {
		return token, nil
	}
	if idx < 1 || idx > len(discoveredRoots) {
		return "", fmt.Errorf("%q is not a valid root number: only 1-%d are listed above", token, len(discoveredRoots))
	}
	return discoveredRoots[idx-1], nil
}

// promptForRoots prints discovered.Roots/Candidates as a suggestion, then
// repeatedly reads one line of the user's own answer until it resolves to a
// non-empty set of valid repository-relative directories -- the same
// explain-and-retry-or-cancel treatment every other field (layers, forbidden
// pairs, required layer) already gets. There is no answer that selects zero
// roots: validateRoots (the same schema validator the write
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

		answer, _ := prompt.ReadLine(reader)
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
