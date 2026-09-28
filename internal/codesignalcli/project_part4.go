package codesignalcli

import (
	"fmt"
)

func validateProjectConfigRoots(roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("roots must contain at least one repository-relative directory")
	}
	if len(roots) > maxProjectConfigRoots {
		return fmt.Errorf("roots exceed budget of %d entries", maxProjectConfigRoots)
	}
	for _, root := range roots {
		if err := validateProjectConfigDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}

	if hasDuplicatePaths(roots) {
		return fmt.Errorf("roots must be unique")
	}
	return nil
}

// parseProjectConfig performs validateProjectConfigJSON's full decode and
// schema validation, additionally returning the decoded projectConfig. It
// exists so a caller that needs the decoded value (checkPolicy, via
// loadProjectConfigForReadiness) never has to run a second, redundant decode
// of bytes validateProjectConfigJSON already accepted.
func parseProjectConfig(data []byte) (projectConfig, error) {
	config, err := decodeProjectConfig(data)
	if err != nil {
		return projectConfig{}, err
	}
	if err := validateProjectConfigRoots(config.Roots); err != nil {
		return projectConfig{}, err
	}
	seenLayerNames, err := validateProjectConfigLayers(config.Layers)
	if err != nil {
		return projectConfig{}, err
	}
	if err := validateProjectConfigForbiddenImports(config.ForbiddenImports, seenLayerNames); err != nil {
		return projectConfig{}, err
	}
	if err := validateProjectConfigCrossFields(config, seenLayerNames); err != nil {
		return projectConfig{}, err
	}
	return config, nil
}
