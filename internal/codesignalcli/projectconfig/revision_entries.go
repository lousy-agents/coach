package projectconfig

import (
	"path"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// revisionEntries lists the names directly under a directory of the analyzed
// revision's tree, the empty string being the root. A directory the revision
// does not hold has none.
type revisionEntries func(directory string) ([]string, error)

// entriesThrough asks run, in dir, about the tree of revision.
func entriesThrough(run gitrepo.Runner, dir, revision string) revisionEntries {
	return func(directory string) ([]string, error) {
		return gitrepo.DirectoryEntriesAtRevision(run, dir, revision, directory)
	}
}

// holds reports whether the revision has an entry of any type at entry,
// spelled exactly as given.
func (e revisionEntries) holds(entry string) (bool, error) {
	parent := path.Dir(entry)
	if parent == "." {
		parent = ""
	}
	names, err := e(parent)
	if err != nil {
		return false, err
	}
	return slices.Contains(names, path.Base(entry)), nil
}

// holdsPath reports whether the revision has an entry at every prefix of entry, each spelled exactly as given.
func (e revisionEntries) holdsPath(entry string) (bool, error) {
	directory := ""
	for _, part := range strings.Split(entry, "/") {
		names, err := e(directory)
		if err != nil {
			return false, err
		}
		if !slices.Contains(names, part) {
			return false, nil
		}
		directory = path.Join(directory, part)
	}
	return true, nil
}
