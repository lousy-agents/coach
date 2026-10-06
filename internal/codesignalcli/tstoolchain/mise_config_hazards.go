package tstoolchain

import (
	"strings"
)

// HasMiseConfigHazard is a sibling scan to miseToolsSectionLines: instead of
// collecting [tools] content, it flags the mere presence, anywhere in the
// file (including sections miseToolsSectionLines skips), of the
// execution/redirection constructs a repository-controlled mise config could
// use against Coach: a [hooks] table (mise runs its commands on shell
// activation or tool install), a [tasks] table (an auto-run task, the same
// execution-hazard class as [hooks]), a [registry] override (redirects
// where a tool resolves from), an env `_.source` directive (sources another
// file's environment into this config's context), or a [tools] entry whose
// value is an inline table naming a tool-level `postinstall`/`backend`/`url`
// override, or a bare top-level `hooks`/`tasks`/`registry` key assigned an
// inline table (mise's shorthand for those tables without the bracketed
// header). Header/key matching is whitespace- and quote-tolerant
// (`[ hooks ]`, `[ tasks ]`, `[[ registry.x ]]`, `["hooks"]`, `['hooks']`)
// because mise itself accepts those spellings. The command or destination
// inside is never evaluated -- presence alone is the signal, so this stays
// a pure scan over already-read bytes with no TOML semantics beyond section
// headers.
func HasMiseConfigHazard(data string) bool {
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "[["):
			if hazardousHeader(line, "[[", "registry") {
				return true
			}
		case strings.HasPrefix(line, "["):
			if hazardousHeader(line, "[", "hooks", "tasks", "registry") {
				return true
			}
		default:
			if miseLineDeclaresEnvSource(line) || miseLineDeclaresToolExecutionOverride(line) || miseLineDeclaresHazardousInlineTable(line) {
				return true
			}
		}
	}
	return false
}

// hazardousHeader strips prefix and any TOML quoting from a section-header
// line, then reports whether what remains starts with one of names.
func hazardousHeader(line, prefix string, names ...string) bool {
	header := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"'`)
	for _, name := range names {
		if strings.HasPrefix(header, name) {
			return true
		}
	}
	return false
}

func miseLineDeclaresEnvSource(line string) bool {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	return strings.Trim(strings.TrimSpace(key), `"'`) == "_.source"
}
