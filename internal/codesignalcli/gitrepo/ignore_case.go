package gitrepo

import "strings"

// IgnoresCase reports whether git treats names that differ only in case as one, which is what
// core.ignorecase says and what git init sets on a file system that folds case. On such a repository
// `git add` cannot record a rename that changes only the case of a name; `git mv` can.
func IgnoresCase(run Runner, dir string) bool {
	output, err := run(dir, "config", "--type=bool", "--get", "core.ignorecase")
	return err == nil && strings.TrimSpace(string(output)) == "true"
}
