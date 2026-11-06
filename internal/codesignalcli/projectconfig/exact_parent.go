package projectconfig

import (
	"errors"
)

// spellingBeforeMissing is err, unless err says the file is absent while a parent
// is spelled in another case. A restore writes where the file system resolves the
// parents, so it would put the file into a directory the scan then refuses; the
// parent's spelling is what the user has to fix, so that is what is reported.
func spellingBeforeMissing(root diskRoot, repoPath string, err error) error {
	var found *ConfigError
	if !errors.As(err, &found) || found.Kind != KindMissing {
		return err
	}
	if spelling := checkExactParents(root, repoPath); spelling != nil {
		return spelling
	}
	return err
}
