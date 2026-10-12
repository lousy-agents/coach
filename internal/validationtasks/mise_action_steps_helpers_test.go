package validationtasks

import (
	"strings"
)

func miseTomlMinVersion(toml string) string {
	m := miseTomlMinVersionPattern.FindStringSubmatch(toml)
	if m == nil {
		return ""
	}
	return m[1]
}

func miseActionStepBodies(yml string) []string {
	lines := strings.Split(yml, "\n")
	var bodies []string
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "- uses:") || !strings.Contains(trimmed, "jdx/mise-action@") {
			continue
		}
		indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
		end := indentedBlockEnd(lines, i+1, indent)

		bodies = append(bodies, strings.Join(lines[i:end], "\n"))
		i = end - 1
	}
	return bodies
}

// indentedBlockEnd returns the index just past the run of lines from start
// that are blank or indented deeper than indent: the body of the YAML list
// item whose dash sits at indent.
func indentedBlockEnd(lines []string, start, indent int) int {
	end := start
	for end < len(lines) {
		if strings.TrimSpace(lines[end]) == "" {
			end++
			continue
		}
		lineIndent := len(lines[end]) - len(strings.TrimLeft(lines[end], " \t"))
		if lineIndent <= indent {
			break
		}
		end++
	}
	return end
}

func miseActionVersion(step string) (string, bool) {
	for _, line := range strings.Split(step, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, "version:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		value = strings.Trim(value, `"'`)
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}
