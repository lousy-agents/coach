package pkgmanager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// readPackageManagerField reads package.json's Corepack-style
// "packageManager": "<name>@<version>" field. An unrecognized manager name
// is treated the same as an absent field, falling back to lockfile
// detection instead.
func readPackageManagerField(root string) (kind, version string, ok bool) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "", "", false
	}
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.PackageManager == "" {
		return "", "", false
	}
	name, rest, found := strings.Cut(manifest.PackageManager, "@")
	if !found || rest == "" {
		return "", "", false
	}
	switch name {
	case KindNPM, KindPNPM, KindBun, KindYarn:
		return name, rest, true
	default:
		return "", "", false
	}
}

// trimConfigValueQuotes strips a single layer of matching double or single
// quotes from a config value, per .npmrc's ini quoting rules -- also
// sufficient for a TOML basic/literal string's outer quotes in
// detectBunfigHazard's narrow scan.
func trimConfigValueQuotes(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
