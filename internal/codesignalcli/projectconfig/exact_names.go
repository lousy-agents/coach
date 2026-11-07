package projectconfig

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// checkExactNames requires every component of the policy path to be spelled on
// disk as the path spells it. A filesystem that folds case resolves a different
// spelling to the same file, while the analyzed revision holds only the exact
// one, so the scan would read a committed policy the working tree does not
// hold under that name. Only a case difference is refused: a name that no
// entry matches is left to the other checks, because a Unicode normalization
// difference is not one.
func checkExactNames(root diskRoot, repoPath string) error {
	return checkExactNamesThrough(root, repoPath, strings.Count(repoPath, "/")+1)
}

// checkExactParents is checkExactNames for the directories above the file alone.
// A file that is not there has no spelling to compare, and where the file system
// tells cases apart a name in another case is another file.
func checkExactParents(root diskRoot, repoPath string) error {
	return checkExactNamesThrough(root, repoPath, strings.Count(repoPath, "/"))
}

// checkExactNamesThrough checks the first count components of the path.
func checkExactNamesThrough(root diskRoot, repoPath string, count int) error {
	parts := strings.Split(repoPath, "/")
	for i, part := range parts[:count] {
		directory := "."
		if i > 0 {
			directory = strings.Join(parts[:i], "/")
		}
		spelled, err := spellingIn(root, directory, part)
		if err != nil {
			return unusableOnDiskError(repoPath, projectreadiness.CauseUnreadableOnDisk, "cannot be checked on disk: "+causeText(err))
		}
		if spelled != part {
			return newCaseMismatch(repoPath, parts, i, spelled)
		}
	}
	return nil
}
