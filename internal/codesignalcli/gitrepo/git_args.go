package gitrepo

// gitArgs is the argument list every git process of this package starts with.
//
// --no-optional-locks keeps a read from maintaining the user's repository: without it `git status`
// refreshes stat information and writes .git/index back, which takes the index lock (so it can make
// the user's own concurrent git fail) and runs a post-index-change hook the repository carries.
//
// core.fsmonitor=false stops git from running the fsmonitor program that a repository's config, or a
// GIT_CONFIG_* variable, names: left on, a read-only scan would execute whatever that program is.
func gitArgs(dir string, args ...string) []string {
	return append([]string{"--no-optional-locks", "-C", dir, "-c", "core.fsmonitor=false"}, args...)
}
