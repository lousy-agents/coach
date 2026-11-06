package gitrepo

import "strings"

// DirectoryEntriesAtRevision lists the names directly under directory in
// revision's tree, the empty string being the root. A directory the tree does
// not hold, a name in another case, and a path that is not a directory all
// have none. It reads no object and does not check that revision resolves: a
// caller that compares spellings and treats a failure as "cannot tell" has no
// use for either. ls-tree matches a name exactly whatever core.ignorecase says,
// so the names are the revision's own spellings.
func DirectoryEntriesAtRevision(run Runner, dir, revision, directory string) ([]string, error) {
	args := []string{"ls-tree", "--full-tree", "--name-only", "-z", revision}
	if directory != "" {
		args = append(args, "--", ":(top,literal)"+directory+"/")
	}
	output, err := run(dir, args...)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range strings.Split(string(output), "\x00") {
		if entry != "" {
			names = append(names, entry[strings.LastIndex(entry, "/")+1:])
		}
	}
	return names, nil
}
