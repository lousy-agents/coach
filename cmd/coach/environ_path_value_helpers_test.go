package main

import (
	"strings"
)

// pathValueFromEnviron returns the value of the variable named exactly PATH
// from `env` output, one NAME=value per line.
func pathValueFromEnviron(env string) string {
	for _, line := range strings.Split(env, "\n") {
		if value, ok := strings.CutPrefix(line, "PATH="); ok {
			return value
		}
	}
	return ""
}
