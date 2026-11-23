package main

import (
	"strings"
)

// unset, as an override, removes the variable: a GOROOT that names the root
// user's toolchain would send the non-root go to a directory it cannot read.
const unset = "\x00unset"

// inheritedGitConfig names the variables that carry git configuration or an
// identity in the environment itself. CI's runner sets none of them, so a
// test that passes only because one is set here would fail there.
var inheritedGitConfig = []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"}

func withoutInheritedGitConfig(environ []string) []string {
	var kept []string
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !isInheritedGitConfig(name) {
			kept = append(kept, entry)
		}
	}
	return kept
}

func isInheritedGitConfig(name string) bool {
	if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
		return true
	}
	for _, listed := range inheritedGitConfig {
		if name == listed {
			return true
		}
	}
	return false
}

func withOverrides(environ []string, overrides map[string]string) []string {
	var out []string
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrides[name]; !overridden {
			out = append(out, entry)
		}
	}
	for name, value := range overrides {
		if value != unset {
			out = append(out, name+"="+value)
		}
	}
	return out
}
