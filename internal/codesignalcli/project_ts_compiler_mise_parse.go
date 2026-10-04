package codesignalcli

import "strings"

const miseToolsSectionHeader = "[tools]"

// parseMiseToolsTypescriptVersions is a line-based scan, not a TOML parse:
// every section other than [tools] -- [hooks], [tasks], env directives --
// must be skipped unread rather than interpreted.

func typescriptVersionsFromMiseToolLine(line string) []string {
	key, value, ok := strings.Cut(line, "=")
	if !ok || strings.Trim(strings.TrimSpace(key), `"'`) != miseNpmTypescriptTool {
		return nil
	}
	return parseMiseToolValue(strings.TrimSpace(value))
}

// hasMiseConfigHazard is a sibling scan to miseToolsSectionLines: instead of
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
func hasMiseConfigHazard(data string) bool {
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

// miseLineDeclaresHazardousInlineTable flags a bare top-level `hooks = { ... }`,
// `tasks = { ... }`, or `registry = { ... }` key -- mise's inline-table
// shorthand for declaring those tables without the bracketed `[hooks]`/
// `[tasks]`/`[[registry.*]]` header syntax the two bracket-prefixed cases
// above already catch.

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
