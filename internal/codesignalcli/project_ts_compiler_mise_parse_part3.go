package codesignalcli

import (
	"strings"
)

func unquoteMiseString(value string) (string, bool) {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1], true
	}
	return "", false
}
func miseLineDeclaresEnvSource(line string) bool {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return false
	}
	return strings.Trim(strings.TrimSpace(key), `"'`) == "_.source"
}

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
