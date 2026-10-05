package codesignalcli

import (
	"fmt"
	"io"
	"strings"
)

func printCandidateSummary(out io.Writer, roots []string, layers []projectConfigLayer, forbidden []projectForbiddenImport, requiredLayer string) {
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

func validateForbiddenPairCandidate(from, to string, layers []projectConfigLayer, existing []projectForbiddenImport) error {
	if from == "" || to == "" {
		return fmt.Errorf("forbidden import pairs require a non-empty source and destination layer")
	}
	if !layerNameDeclared(from, layers) {
		return fmt.Errorf("forbidden import pair references undefined layer %q", from)
	}
	if !layerNameDeclared(to, layers) {
		return fmt.Errorf("forbidden import pair references undefined layer %q", to)
	}
	for _, pair := range existing {
		if pair.From == from && pair.To == to {
			return fmt.Errorf("forbidden import pairs must be unique")
		}
	}
	return nil
}
