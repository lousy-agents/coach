package tstoolchain

import (
	"regexp"
	"strings"
)

// SupportedTypescriptVersions is the compiled-in set of exact TypeScript
// compiler versions this build will load, ascending. TypeScript does not
// follow semantic versioning and the analyzer depends on
// typescript/unstable/* subpaths, so no range is promised.
var SupportedTypescriptVersions = []string{"7.0.2"}

func NewestSupportedTypescriptVersion() string {
	return SupportedTypescriptVersions[len(SupportedTypescriptVersions)-1]
}

func IsSupportedTypescriptVersion(version string) bool {
	for _, candidate := range SupportedTypescriptVersions {
		if candidate == version {
			return true
		}
	}
	return false
}

func SupportedTypescriptVersionsCopy() []string {
	out := make([]string, len(SupportedTypescriptVersions))
	copy(out, SupportedTypescriptVersions)
	return out
}

var exactVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func IsExactVersion(value string) bool {
	return exactVersionPattern.MatchString(strings.TrimSpace(value))
}

func DedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
