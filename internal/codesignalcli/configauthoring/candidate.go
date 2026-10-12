package configauthoring

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// promptForApproval prints the complete candidate together with its coverage
// preview -- each named layer's prefixes paired with the discovered
// directories they match, and every discovered directory no declared layer
// matches -- and then reads one answer. Only the exact approval token
// (case-insensitive) approves; anything else, including a blank answer or an
// exhausted/erroring read, does not. There is no retry here: the point of
// this gate is that the user either approves what was just shown or they
// don't, so a single read is always enough to decide it.
func promptForApproval(out io.Writer, reader *bufio.Reader, discovered projectmodel.TSRootDiscoveryResult, roots []string, layers []projectconfig.Layer, forbidden []projectconfig.ForbiddenImport, requiredLayer string) bool {
	printCandidateSummary(out, roots, layers, forbidden, requiredLayer)
	printCoveragePreview(out, discovered, layers)

	fmt.Fprintln(out, "Type 'approve' to write this project config, or anything else to cancel without writing:")
	fmt.Fprint(out, "> ")
	answer, _ := prompt.ReadLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "approve")
}

func printCandidateSummary(out io.Writer, roots []string, layers []projectconfig.Layer, forbidden []projectconfig.ForbiddenImport, requiredLayer string) {
	fmt.Fprintln(out, "Candidate project config:")
	fmt.Fprintf(out, "  roots: %s\n", formatStringList(roots))
	if len(layers) == 0 {
		fmt.Fprintln(out, "  layers: (none)")
	} else {
		fmt.Fprintln(out, "  layers:")
		for _, layer := range layers {
			fmt.Fprintf(out, "    - %s: %s\n", layer.Name, strings.Join(layer.Prefixes, ", "))
		}
	}
	if len(forbidden) == 0 {
		fmt.Fprintln(out, "  forbidden_imports: (none)")
	} else {
		fmt.Fprintln(out, "  forbidden_imports:")
		for _, pair := range forbidden {
			fmt.Fprintf(out, "    - %s -> %s\n", pair.From, pair.To)
		}
	}
	if requiredLayer == "" {
		fmt.Fprintln(out, "  required_layer: (none)")
	} else {
		fmt.Fprintf(out, "  required_layer: %s\n", requiredLayer)
	}
}

// buildApprovedCandidate renders the collected fields as the schema-1
// project-config document and runs it through projectconfig.Parse -- the same
// validator projectconfig.Load applies to a committed --project-config file
// -- before treating it as valid. source_sink_pack is never populated: it is
// a reserved field this feature must not touch.
func buildApprovedCandidate(roots []string, layers []projectconfig.Layer, forbidden []projectconfig.ForbiddenImport, requiredLayer string) ([]byte, error) {
	candidate := projectconfig.Config{
		SchemaVersion:    "1",
		Roots:            roots,
		Layers:           layers,
		ForbiddenImports: forbidden,
		RequiredLayer:    requiredLayer,
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := projectconfig.Parse(data); err != nil {
		return nil, err
	}
	return data, nil
}
