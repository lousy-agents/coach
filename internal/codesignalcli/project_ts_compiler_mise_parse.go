package codesignalcli

import "strings"

const miseToolsSectionHeader = "[tools]"

// parseMiseToolsTypescriptVersions is a line-based scan, not a TOML parse:
// every section other than [tools] -- [hooks], [tasks], env directives --
// must be skipped unread rather than interpreted.
func parseMiseToolsTypescriptVersions(data string) []string {
	var versions []string
	for _, line := range miseToolsSectionLines(data) {
		versions = append(versions, typescriptVersionsFromMiseToolLine(line)...)
	}
	return versions
}

func miseToolsSectionLines(data string) []string {
	var lines []string
	inTools := false
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "["):
			inTools = strings.HasPrefix(line, miseToolsSectionHeader)
		case inTools:
			lines = append(lines, line)
		}
	}
	return lines
}

func typescriptVersionsFromMiseToolLine(line string) []string {
	key, value, ok := strings.Cut(line, "=")
	if !ok || strings.Trim(strings.TrimSpace(key), `"'`) != miseNpmTypescriptTool {
		return nil
	}
	return parseMiseToolValue(strings.TrimSpace(value))
}

func parseMiseToolValue(value string) []string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return parseMiseToolArray(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	}
	if unquoted, ok := unquoteMiseString(value); ok {
		return []string{unquoted}
	}
	return nil
}

func parseMiseToolArray(inner string) []string {
	var values []string
	for _, part := range strings.Split(inner, ",") {
		if unquoted, ok := unquoteMiseString(strings.TrimSpace(part)); ok {
			values = append(values, unquoted)
		}
	}
	return values
}

func unquoteMiseString(value string) (string, bool) {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1], true
	}
	return "", false
}

// hasMiseConfigHazard is a sibling scan to miseToolsSectionLines: instead of
// collecting [tools] content, it flags the mere presence, anywhere in the
// file (including sections miseToolsSectionLines skips), of the
// execution/redirection constructs a repository-controlled mise config could
// use against Coach (AC-9/AC-SET-12): a [hooks] table (mise runs its
// commands on shell activation or tool install), a [registry] override
// (redirects where a tool resolves from), an env `_.source` directive
// (sources another file's environment into this config's context), or a
// [tools] entry whose value is an inline table naming a tool-level
// `postinstall`/`backend`/`url` override, or a bare top-level `hooks`/
// `registry` key assigned an inline table (mise's shorthand for the same two
// tables without the bracketed header). Header/key matching is whitespace-
// and quote-tolerant (`[ hooks ]`, `[[ registry.x ]]`, `["hooks"]`,
// `['hooks']`) because mise itself accepts those spellings. The command or
// destination inside is never evaluated -- presence alone is the signal, so
// this stays a pure scan over already-read bytes with no TOML semantics
// beyond section headers.
func hasMiseConfigHazard(data string) bool {
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "[["):
			header := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "[[")), `"'`)
			if strings.HasPrefix(header, "registry") {
				return true
			}
		case strings.HasPrefix(line, "["):
			header := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "[")), `"'`)
			if strings.HasPrefix(header, "hooks") || strings.HasPrefix(header, "registry") {
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

// miseLineDeclaresHazardousInlineTable flags a bare top-level `hooks = { ... }`
// or `registry = { ... }` key -- mise's inline-table shorthand for declaring a
// hooks/registry table without the bracketed `[hooks]`/`[[registry.*]]`
// header syntax the two bracket-prefixed cases above already catch.
func miseLineDeclaresHazardousInlineTable(line string) bool {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	key = strings.Trim(strings.TrimSpace(key), `"'`)
	if key != "hooks" && key != "registry" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(value), "{")
}

func miseLineDeclaresEnvSource(line string) bool {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	return strings.Trim(strings.TrimSpace(key), `"'`) == "_.source"
}

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
