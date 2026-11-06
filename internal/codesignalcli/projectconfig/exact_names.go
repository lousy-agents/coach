package projectconfig

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const entryListingBatch = 256

// checkExactNames requires every component of the policy path to be spelled on
// disk as the path spells it. A filesystem that folds case resolves a different
// spelling to the same file, while the analyzed revision holds only the exact
// one, so the scan would read a committed policy the working tree does not
// hold under that name. Only a case difference is refused: a name that no
// entry matches is left to the other checks, because a Unicode normalization
// difference is not one.
func checkExactNames(root *os.Root, repoPath string) error {
	parts := strings.Split(repoPath, "/")
	for i, part := range parts {
		directory := "."
		if i > 0 {
			directory = strings.Join(parts[:i], "/")
		}
		spelled, err := spellingIn(root, directory, part)
		if err != nil {
			return unusableOnDiskError(repoPath, "cannot be checked on disk: "+causeText(err))
		}
		if spelled != part {
			return unusableOnDiskError(repoPath, fmt.Sprintf("is stored as %q on disk, not %q; this filesystem ignores case, so the name resolves, but the analyzed revision holds only the exact spelling", spelled, part))
		}
	}
	return nil
}

// spellingIn is the entry of directory spelled exactly as name, or else the
// first entry that differs from it only by case, or else name itself.
func spellingIn(root *os.Root, directory, name string) (string, error) {
	listing, err := root.Open(directory)
	if err != nil {
		return "", err
	}
	defer func() { _ = listing.Close() }()
	caseVariant := name
	for {
		names, err := listing.Readdirnames(entryListingBatch)
		for _, entry := range names {
			if entry == name {
				return name, nil
			}
			if caseVariant == name && strings.EqualFold(entry, name) {
				caseVariant = entry
			}
		}
		if err == io.EOF {
			return caseVariant, nil
		}
		if err != nil {
			return "", err
		}
	}
}
