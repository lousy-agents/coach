package codesignalcli

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func validateProjectConfigForbiddenImports(forbiddenImports []projectForbiddenImport, seenLayerNames map[string]struct{}) error {
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

func decodeProjectConfig(data []byte) (projectConfig, error) {
	if int64(len(data)) > maxProjectConfigBytes {
		return projectConfig{}, fmt.Errorf("document exceeds %d-byte size budget", maxProjectConfigBytes)
	}
	if !json.Valid(data) {
		return projectConfig{}, fmt.Errorf("document is not valid JSON")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return projectConfig{}, err
	}

	var config projectConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return projectConfig{}, fmt.Errorf("schema decode failed: %s", err)
	}
	if config.SchemaVersion != "1" {
		return projectConfig{}, fmt.Errorf("schema_version must be \"1\"")
	}
	return config, nil
}
