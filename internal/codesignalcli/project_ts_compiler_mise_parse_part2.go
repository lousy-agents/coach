package codesignalcli

import (
	"strings"
)

func parseMiseToolArray(inner string) []string {
	var values []string
	for _, part := range strings.Split(inner, ",") {
		if unquoted, ok := unquoteMiseString(strings.TrimSpace(part)); ok {
			values = append(values, unquoted)
		}
	}
	return values
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
func parseMiseToolValue(value string) []string {
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return parseMiseToolArray(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	}
	if unquoted, ok := unquoteMiseString(value); ok {
		return []string{unquoted}
	}
	return nil
}
