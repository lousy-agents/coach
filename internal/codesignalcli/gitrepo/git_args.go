package gitrepo

// gitArgs is the argument list every git process of this package starts with: the directory to
// work in, and core.fsmonitor switched off. core.fsmonitor names a program that git runs on every
// command that refreshes the index, so leaving it to the repository's config, or to a
// GIT_CONFIG_* variable, would let the repository under analysis execute code during a read-only scan.
func gitArgs(dir string, args ...string) []string {
	return append([]string{"-C", dir, "-c", "core.fsmonitor=false"}, args...)
}
