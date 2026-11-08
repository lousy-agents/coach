package projectconfig

import (
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
