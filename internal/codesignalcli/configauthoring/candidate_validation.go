package configauthoring

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func validateLayerPrefixCandidate(name string, prefixes []string, existing []projectconfig.Layer) error {
	if len(prefixes) == 0 {
		return fmt.Errorf("layer %q must contain at least one prefix", name)
	}
	var allPrefixes []string
	for _, layer := range existing {
		allPrefixes = append(allPrefixes, layer.Prefixes...)
	}
	for _, prefix := range prefixes {
		if err := projectconfig.ValidateDirectory(prefix); err != nil {
			return fmt.Errorf("prefix %q: %s", prefix, err)
		}
		allPrefixes = append(allPrefixes, prefix)
	}
	if len(allPrefixes) > projectconfig.MaxLayerPrefixes {
		return fmt.Errorf("layer prefixes exceed budget of %d entries", projectconfig.MaxLayerPrefixes)
	}
	if projectconfig.HasDuplicateOrOverlappingPaths(allPrefixes) {
		return fmt.Errorf("layer prefixes must be unique and non-overlapping across all layers")
	}
	return nil
}

func validateForbiddenPairCandidate(from, to string, layers []projectconfig.Layer, existing []projectconfig.ForbiddenImport) error {
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
