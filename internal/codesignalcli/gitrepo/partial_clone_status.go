package gitrepo

// mayBeAPartialClone is whether a git that cannot be told not to fetch could fetch while it compares a staged
// deletion with a staged addition. Only a partial clone lacks objects that a fetch would supply, so a repository that
// is not one is safe to compare; a repository whose configuration cannot be read is taken to be one.
func mayBeAPartialClone(run Runner, dir string, canBeToldNotToFetch bool) bool {
	if canBeToldNotToFetch {
		return false
	}
	_, found, err := promisorRemote(run, dir)
	return found || err != nil || hasPartialCloneFilter(run, dir)
}

// hasPartialCloneFilter is whether a remote has a partial clone filter, which makes git fetch missing objects
// from it even with its promisor setting unset. A configuration git cannot be asked about is taken to have one.
func hasPartialCloneFilter(run Runner, dir string) bool {
	output, err := run(dir, "config", "--get-regexp", `^remote\..*\.partialclonefilter$`)
	if err != nil {
		return !ConfigHasNoValue(err)
	}
	return len(output) > 0
}
