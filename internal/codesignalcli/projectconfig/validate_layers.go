package projectconfig

import (
	"fmt"
)

// validateLayers returns the declared layer names so
// validateForbiddenImports and validateLayerReferences
// can check their own layer references against it.
func validateLayers(layers []Layer) (map[string]struct{}, error) {
	seenLayerNames := make(map[string]struct{}, len(layers))
	var allPrefixes []string
	for _, layer := range layers {
		if layer.Name == "" {
			return nil, fmt.Errorf("layer name must be non-empty")
		}
		if _, exists := seenLayerNames[layer.Name]; exists {
			return nil, fmt.Errorf("layer names must be unique")
		}
		seenLayerNames[layer.Name] = struct{}{}
		prefixes, err := collectLayerPrefixes(layer)
		if err != nil {
			return nil, err
		}
		allPrefixes = append(allPrefixes, prefixes...)
	}
	if len(allPrefixes) > MaxLayerPrefixes {
		return nil, fmt.Errorf("layer prefixes exceed budget of %d entries", MaxLayerPrefixes)
	}
	if HasDuplicateOrOverlappingPaths(allPrefixes) {
		return nil, fmt.Errorf("layer prefixes must be unique and non-overlapping")
	}
	return seenLayerNames, nil
}

func collectLayerPrefixes(layer Layer) ([]string, error) {
	if len(layer.Prefixes) == 0 {
		return nil, fmt.Errorf("layer %q must contain at least one prefix", layer.Name)
	}
	var prefixes []string
	for _, prefix := range layer.Prefixes {
		if err := ValidateDirectory(prefix); err != nil {
			return nil, fmt.Errorf("layer %q prefix %q: %s", layer.Name, prefix, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
