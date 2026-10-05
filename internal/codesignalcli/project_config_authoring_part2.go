package codesignalcli

import (
	"fmt"
)

func validateLayerPrefixCandidate(name string, prefixes []string, existing []projectConfigLayer) error {
	if len(prefixes) == 0 {
		return fmt.Errorf("layer %q must contain at least one prefix", name)
	}
	var allPrefixes []string
	for _, layer := range existing {
		allPrefixes = append(allPrefixes, layer.Prefixes...)
	}
	for _, prefix := range prefixes {
		if err := validateProjectConfigDirectory(prefix); err != nil {
			return fmt.Errorf("prefix %q: %s", prefix, err)
		}
		allPrefixes = append(allPrefixes, prefix)
	}
	if len(allPrefixes) > maxProjectConfigLayerPrefixes {
		return fmt.Errorf("layer prefixes exceed budget of %d entries", maxProjectConfigLayerPrefixes)
	}
	if hasDuplicateOrOverlappingPaths(allPrefixes) {
		return fmt.Errorf("layer prefixes must be unique and non-overlapping across all layers")
	}
	return nil
}
func validateRootSelection(selected []string) error {
	if len(selected) == 0 {
		return fmt.Errorf("at least one repository-relative root must be selected")
	}
	if len(selected) > maxProjectConfigRoots {
		return fmt.Errorf("roots exceed budget of %d entries", maxProjectConfigRoots)
	}
	for _, root := range selected {
		if err := validateProjectConfigDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}
	return nil
}
