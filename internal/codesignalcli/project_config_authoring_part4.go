package codesignalcli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
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

// promptForLayers collects named layers and their prefixes, one at a time,
// until the user leaves a layer name blank. It never suggests a name or a
// prefix on the user's behalf: every layer in the returned slice came from
// the user's own typed answers. Each candidate is checked with the same
// name-uniqueness and prefix-overlap rules the frozen project-config schema
// itself enforces (validateProjectConfigLayers), so a mistake is caught and
// explained here rather than deferred to a later validation pass.
func promptForLayers(out io.Writer, reader *bufio.Reader) (layers []projectConfigLayer, cancelled bool) {
	fmt.Fprintln(out, "Define named layers for architecture-boundary policy. Each layer needs a name and one or more repository-relative path prefixes.")
	for {
		name, done, cancelled := promptLayerName(out, reader, layers)
		if cancelled {
			return layers, true
		}
		if done {
			return layers, false
		}

		prefixes, cancelled := promptLayerPrefixes(out, reader, name, layers)
		if cancelled {
			return layers, true
		}
		layers = append(layers, projectConfigLayer{Name: name, Prefixes: prefixes})
	}
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
