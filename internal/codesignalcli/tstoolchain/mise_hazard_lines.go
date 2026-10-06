package tstoolchain

import (
	"strings"
)

// miseLineDeclaresToolExecutionOverride flags an inline-table value (mise's
// `tool = { ... }` shorthand, used both for [tools] entries with
// per-tool options and for hook tables) that names postinstall/backend/url:
// a tool-level postinstall runs a command on that tool's install, and
// backend/url override where/how mise fetches it instead of the default npm
// resolution.
func miseLineDeclaresToolExecutionOverride(line string) bool {
	_, value, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{") {
		return false
	}
	for _, hazardKey := range []string{"postinstall", "backend", "url"} {
		if strings.Contains(value, hazardKey+" ") || strings.Contains(value, hazardKey+"=") {
			return true
		}
	}
	return false
}

// miseLineDeclaresHazardousInlineTable flags a bare top-level `hooks = { ... }`,
// `tasks = { ... }`, or `registry = { ... }` key -- mise's inline-table
// shorthand for declaring those tables without the bracketed `[hooks]`/
// `[tasks]`/`[[registry.*]]` header syntax the two bracket-prefixed cases
// above already catch.
func miseLineDeclaresHazardousInlineTable(line string) bool {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	key = strings.Trim(strings.TrimSpace(key), `"'`)
	if key != "hooks" && key != "tasks" && key != "registry" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(value), "{")
}
