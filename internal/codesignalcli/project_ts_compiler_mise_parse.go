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
	if !ok || strings.Trim(strings.TrimSpace(key), `"'`) != "npm:typescript" {
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
