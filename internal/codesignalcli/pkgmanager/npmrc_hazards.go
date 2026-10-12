package pkgmanager

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// isNpmrcRegistryKey reports whether key is .npmrc's global "registry"
// setting or a scoped "@scope:registry" override. Shared between npm's
// detectNpmrcHazard and detectNpmrcRegistryHazard (pnpm and Bun), since both
// read the same .npmrc format npm does for their own registry resolution.
func isNpmrcRegistryKey(key string) bool {
	return key == "registry" || strings.HasSuffix(key, ":registry")
}

// scanNpmrcLines reads root's committed .npmrc (the ini-style config file
// both npm and pnpm honor -- verified empirically that `pnpm config get`
// resolves a committed .npmrc's keys the same way npm does) and calls handle
// with each non-comment, non-blank line's lowercased key and unquoted value,
// stopping at the first non-empty detail handle returns. Absence of .npmrc
// is safe ("" with handle never called); a .npmrc that exists but cannot be
// read (permission denied, a directory, a dangling symlink) is a hazard in
// its own right, distinct from no .npmrc existing at all -- fail-closed
// rather than treating an unreadable hazard file as absent.
func scanNpmrcLines(root string, handle func(key, value string) string) string {
	path := filepath.Join(root, ".npmrc")
	data, err := os.ReadFile(path)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
			return ""
		}
		return "committed .npmrc could not be read"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = trimConfigValueQuotes(strings.TrimSpace(value))
		if detail := handle(key, value); detail != "" {
			return detail
		}
	}
	return ""
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

// detectNpmrcHazard reports a hazard detail for a committed .npmrc that
// redirects the registry, re-enables lifecycle scripts, or overrides the
// script shell -- npm's full Hazards column (SA-280-012).
func detectNpmrcHazard(root string) string {
	return scanNpmrcLines(root, func(key, value string) string {
		switch {
		case isNpmrcRegistryKey(key):
			return "committed .npmrc redirects the package registry (" + key + "=" + value + ")"
		case key == "ignore-scripts":
			if !strings.EqualFold(value, "true") {
				return "committed .npmrc re-enables lifecycle scripts (ignore-scripts=" + value + ")"
			}
		case key == "script-shell":
			return "committed .npmrc overrides the lifecycle script shell (script-shell=" + value + ")"
		}
		return ""
	})
}
