package codesignalcli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type packageJSONManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// readPackageJSONTypescriptFields distinguishes a missing manifest (no
// fields, unreadable=false) from one that exists and cannot be parsed or
// opened, which the caller must never report as absent.
func readPackageJSONTypescriptFields(dir string) (fields map[string]string, unreadable bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false
		}
		return nil, true
	}
	var manifest packageJSONManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, true
	}
	fields = map[string]string{}
	if value, ok := manifest.Dependencies["typescript"]; ok {
		fields["dependencies"] = value
	}
	if value, ok := manifest.DevDependencies["typescript"]; ok {
		fields["devDependencies"] = value
	}
	return fields, false
}

func readInstalledTypescriptVersion(dir string) (version string, exists bool, unreadable bool) {
	return readTypescriptVersionAt(filepath.Join(dir, "node_modules", "typescript"))
}

func readTypescriptVersionAt(packageDir string) (version string, exists bool, unreadable bool) {
	data, err := os.ReadFile(filepath.Join(packageDir, "package.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, false
		}
		return "", false, true
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return "", false, true
	}
	return manifest.Version, true, false
}
