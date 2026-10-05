package codesignalcli

import (
	"bytes"
	"os"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, path) {
			return true
		}
	}
	return false
}

// resolveTSConfigExtendsTarget retries with ".json" when the literal path is missing.
func resolveTSConfigExtendsTarget(target string) string {
	if strings.HasSuffix(target, ".json") {
		return target
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		return target
	}
	return target + ".json"
}

// trailingCommaFollowedByClose reports whether the comma at data[i] is
// followed only by whitespace before a closing '}' or ']', making it a
// JSONC trailing comma to drop rather than emit.
func trailingCommaFollowedByClose(data []byte, i int) bool {
	j := i + 1
	for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
		j++
	}
	return j < len(data) && (data[j] == '}' || data[j] == ']')
}

func classifyFilename(file gitrepo.SelectedFile) string {
	if strings.HasSuffix(file.Path, "_test.go") {
		return SourceScopeTestOnly
	}
	return SourceScopeUnknown
}

// advanceInsideJSONString writes b (already known to be inside a JSON
// string literal) to out and returns the string/escape state after
// consuming it. Shared by stripJSONCComments and stripTrailingCommas so
// neither strips a comment- or comma-like byte that only appears inside a
// string value.
func advanceInsideJSONString(out *bytes.Buffer, b byte, escaped bool) (stillInString, stillEscaped bool) {
	out.WriteByte(b)
	switch {
	case escaped:
		return true, false
	case b == '\\':
		return true, true
	case b == '"':
		return false, false
	}
	return true, false
}

// skipJSONCLineComment returns the index of the '\n' terminating the "//"
// comment starting at data[i], or len(data) if it runs to EOF.
func skipJSONCLineComment(data []byte, i int) int {
	for i < len(data) && data[i] != '\n' {
		i++
	}
	return i
}
