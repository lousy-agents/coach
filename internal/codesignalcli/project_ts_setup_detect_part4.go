package codesignalcli

import (
	"encoding/json"

	"os"
	"path/filepath"
	"strings"
)

// bunfigRedirectHazard names the redirection it can read directly out of the
// file, walking it as Bun does rather than parsing TOML: a section header
// followed by key/value lines.
func bunfigRedirectHazard(data []byte) string {
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = bunfigSectionName(line)
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = trimConfigValueQuotes(strings.TrimSpace(value))
		switch {
		case section == "install" && key == "registry":
			return "committed bunfig.toml redirects the install registry (registry=" + value + ")"
		case section == "install" && key == "scopes":
			return "committed bunfig.toml redirects scoped install registries (scopes=" + value + ")"
		case section == "install.scopes":
			return "committed bunfig.toml redirects a scoped install registry (" + key + "=" + value + ")"
		}
	}
	return ""
}

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
	case packageManagerKindNPM, packageManagerKindPNPM, packageManagerKindBun, packageManagerKindYarn:
		return name, rest, true
	default:
		return "", "", false
	}
}

// detectNpmrcRegistryHazard reports a hazard detail for a committed .npmrc
// that redirects the package registry, shared between pnpm and Bun's
// detection (verified empirically against pnpm 10.33.0 and Bun 1.3.11: `pnpm
// config get registry` and `bun install --ignore-scripts` both honor a
// committed .npmrc's registry key the same way npm does -- Bun does so even
// with no bunfig.toml present at all). pnpm's own enable-pre-post-scripts
// setting and its onlyBuiltDependencies build-script allowlist were each
// verified empirically, against real pnpm 10.33.0, NOT to re-enable a
// dependency's postinstall script under this package's frozen `pnpm install
// --frozen-lockfile --ignore-scripts --ignore-pnpmfile` argv -- neither is
// checked here, since refusing on a setting that is not an actual bypass
// would be inventing a hazard rather than fail-closed.
func detectNpmrcRegistryHazard(root string) string {
	return scanNpmrcLines(root, func(key, value string) string {
		if isNpmrcRegistryKey(key) {
			return "committed .npmrc redirects the package registry (" + key + "=" + value + ")"
		}
		return ""
	})
}
