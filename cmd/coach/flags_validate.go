package main

import (
	"fmt"
	"sort"
)

func firstDisallowedFlag(setFlags, allowed map[string]bool) (string, bool) {
	for _, name := range sortedFlagNames(setFlags) {
		if !allowed[name] {
			return name, true
		}
	}
	return "", false
}

func sortedFlagNames(setFlags map[string]bool) []string {
	names := make([]string, 0, len(setFlags))
	for name := range setFlags {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func rejectPositionalArgs(flagName string, positional []string, suffix string) string {
	if len(positional) == 0 {
		return ""
	}
	return fmt.Sprintf("coach: --%s does not accept positional arguments%s", flagName, suffix)
}
