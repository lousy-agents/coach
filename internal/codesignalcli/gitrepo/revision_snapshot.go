package gitrepo

import (
	"errors"
	"fmt"
	"os"
)

// ExtractRevision writes the tree of revision to a new temporary directory and returns it.
//
// The files are the committed blobs, byte for byte. `git archive` would not give that: it converts each
// blob on the way out by the committed attributes, which runs a smudge filter named by a .gitattributes
// and defined in the repository's or the user's own configuration (every git-lfs install defines one),
// rewrites line endings and $Format$ keywords, and leaves out a file marked export-ignore. This reads the
// tree with ls-tree and the objects with cat-file, neither of which converts anything, and it does not
// depend on the version of git.
func ExtractRevision(repositoryRoot, revision string) (string, error) {
	listing, err := RunBytes(repositoryRoot, "ls-tree", "-r", "-z", "--full-tree", revision)
	if err != nil {
		return "", fmt.Errorf("reading source snapshot %q: %w", revision, err)
	}
	entries, err := parseSnapshotEntries(listing)
	if err != nil {
		return "", fmt.Errorf("reading source snapshot %q: %w", revision, err)
	}
	dir, err := os.MkdirTemp("", "coach-codesignal-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("creating source snapshot: %w", err)
	}
	if err := writeSnapshot(repositoryRoot, dir, entries); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("reading source snapshot %q: %w", revision, adviseOnMissingObjects(repositoryRoot, revision, err))
	}
	return dir, nil
}

// adviseOnMissingObjects adds the way to fetch to a snapshot that stopped at objects a partial clone lacks.
func adviseOnMissingObjects(repositoryRoot, revision string, err error) error {
	var absent *ObjectsNotLocalError
	if errors.As(err, &absent) {
		absent.Advice = FetchAdvice(RunBytes, repositoryRoot, revision, "")
	}
	return err
}
