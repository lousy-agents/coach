package projectconfig

import (
	"path"
	"slices"

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
