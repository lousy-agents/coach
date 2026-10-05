package projectconfig

import (
	"fmt"
)

func validateForbiddenImports(forbiddenImports []ForbiddenImport, seenLayerNames map[string]struct{}) error {
	seenForbidden := make(map[string]struct{}, len(forbiddenImports))
	for _, forbidden := range forbiddenImports {
		if forbidden.From == "" || forbidden.To == "" {
			return fmt.Errorf("forbidden_imports entries require non-empty from and to")
		}

		// A forbidden_imports entry is an explicit user claim that a layer
		// pair exists. Left unchecked, a typo'd from/to that names no
		// declared layer would validate cleanly but can never match any
		// evaluated (layerFrom, layerTo) pair, silently making that policy
		// line a permanent no-op.
		if _, ok := seenLayerNames[forbidden.From]; !ok {
			return fmt.Errorf("forbidden_imports entry references undefined layer %q", forbidden.From)
		}
		if _, ok := seenLayerNames[forbidden.To]; !ok {
			return fmt.Errorf("forbidden_imports entry references undefined layer %q", forbidden.To)
		}
		key := forbidden.From + "\x00" + forbidden.To
		if _, exists := seenForbidden[key]; exists {
			return fmt.Errorf("forbidden_imports entries must be unique")
		}
		seenForbidden[key] = struct{}{}
	}
	return nil
}

func validateLayerReferences(config Config, seenLayerNames map[string]struct{}) error {
	if config.SourceSinkPack != "" && config.SourceSinkPack != "builtin-v1" {
		return fmt.Errorf("source_sink_pack must be \"builtin-v1\" when supplied")
	}
	if config.RequiredLayer != "" {
		if _, ok := seenLayerNames[config.RequiredLayer]; !ok {
			return fmt.Errorf("required_layer references undefined layer %q", config.RequiredLayer)
		}
	}
	return nil
}
