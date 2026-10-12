package main

import (
	"strings"
)

func pathValueFromEnviron(env string) string {
	for _, part := range strings.FieldsFunc(env, func(r rune) bool { return r == '\n' || r == '\x00' }) {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "PATH=") {
			return strings.TrimSpace(strings.TrimPrefix(part, "PATH="))
		}
		if value, ok := embeddedPathAssignment(part); ok {
			return value
		}
	}
	if i := strings.Index(env, "PATH="); i >= 0 {
		rest := env[i+5:]
		if j := strings.IndexAny(rest, " \n\t"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}

// embeddedPathAssignment reads a PATH= assignment that appears after other
// text within one environ entry, as `ps eww` renders it, up to the next
// space or tab.
func embeddedPathAssignment(part string) (string, bool) {
	i := strings.Index(part, "PATH=")
	if i < 0 {
		return "", false
	}
	rest := part[i+5:]
	if j := strings.IndexAny(rest, " \t"); j >= 0 {
		return rest[:j], true
	}
	return rest, true
}
