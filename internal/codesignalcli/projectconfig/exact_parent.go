package projectconfig

import (
	"errors"
	"strings"
)

// spellingBeforeMissing is err, unless err says the file is absent while a parent
// is spelled in another case. A restore writes where the file system resolves the
// parents, so it would put the file into a directory the scan then refuses; the
// parent's spelling is what the user has to fix, so that is what is reported.
func spellingBeforeMissing(root diskRoot, repoPath string, err error) error {
	return spellingOfExistingBeforeMissing(root, repoPath, strings.Count(repoPath, "/"), err)
}

// spellingOfExistingBeforeMissing is spellingBeforeMissing for the first existing components of the path,
// which is all of the parents when the file alone is missing, and fewer when a deeper directory is missing
// too. A directory that is not there has no spelling to compare, so the check stops short of it; the ones
// above it exist, and a file system that folds case resolves them in whatever case the user typed.
func spellingOfExistingBeforeMissing(root diskRoot, repoPath string, existing int, err error) error {
	var found *ConfigError
	if !errors.As(err, &found) || found.Kind != KindMissing {
		return err
	}
	if spelling := checkExactNamesThrough(root, repoPath, existing); spelling != nil {
		return spelling
	}
	return err
}
