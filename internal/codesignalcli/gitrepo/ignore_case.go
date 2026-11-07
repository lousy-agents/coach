package gitrepo

import "strings"

// IgnoresCase reports whether git treats names that differ only in case as one, which is what
// core.ignorecase says and what git init sets on a file system that folds case. On such a repository
// `git add` cannot record a rename that changes only the case of a name; `git mv` can.
//
// An unset key is a plain no. Any other failure leaves the question unanswered, and the error says so: a
// repository whose answer is unknown must not be taken for one that tells cases apart.
func IgnoresCase(run Runner, dir string) (bool, error) {
	output, err := run(dir, "config", "--type=bool", "--get", "core.ignorecase")
	if err != nil {
		return false, noMatchIsNo(err)
	}
	return strings.TrimSpace(string(output)) == "true", nil
}
