package projectconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// moveGuard says whether a git mv of the committed file from to the path to may be printed for a person to paste.
//
// With core.ignorecase set, git mv skips its check that the destination exists for a move that changes only the case
// of a name, and renames over it. That setting is a record of what the disk was when the repository was made or
// cloned, not of what the disk is: a repository cloned on macOS and copied to a Linux disk keeps it. There, an
// untracked file at the destination is a different file from the committed one, and the move deletes it for good,
// a file that was never in the object store. So a move is printed only where nothing stands at the destination, or
// where what stands there is the committed file itself seen through a disk that folds case.
type moveGuard func(from, to string) bool

// moveGuardFor asks the working tree of the repository dir is in. A root that cannot be found allows no move.
func moveGuardFor(run gitrepo.Runner, dir string) moveGuard {
	return func(from, to string) bool {
		root, err := gitrepo.RepositoryRootWith(run, dir)
		return err == nil && nothingElseStandsAt(root, from, to)
	}
}

// nothingElseStandsAt is whether to is absent under root, or is the same file as from. Anything that cannot be told
// apart from a different file, a path through a file or a directory that cannot be searched among them, is one.
func nothingElseStandsAt(root, from, to string) bool {
	target, err := os.Lstat(filepath.Join(root, filepath.FromSlash(to)))
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	source, err := os.Lstat(filepath.Join(root, filepath.FromSlash(from)))
	return err == nil && os.SameFile(source, target)
}
