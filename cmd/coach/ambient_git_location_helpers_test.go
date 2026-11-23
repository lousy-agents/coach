package main

import (
	"os"
	"strings"
)

var locationVariables = map[string]bool{"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true}

// environmentNamingNoRepository is this process's environment without the variables that say where git's
// repository, work tree and index are, followed by extra, so that a spec that names a layout is not flipped by
// a value the shell or hook that started the suite exported.
func environmentNamingNoRepository(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !locationVariables[name] {
			env = append(env, entry)
		}
	}
	return append(env, extra...)
}
