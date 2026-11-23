package projectconfig

import (
	"errors"
)

// canonicalOnDisk is what the working tree holds at the default policy's path, asked the way a policy named
// with --project-config is asked, so that the scan and --check-project say the same of a path that is not
// committed. A regular file the process can read is a policy that is not committed; nothing there is no
// diagnostic; and anything else (a directory, a link, a file where its directory should be, a path the process
// cannot search or read) is a path that cannot be a policy, and says why. The result carries only the fields of
// Selection that describe the disk.
func canonicalOnDisk(root string) (Selection, error) {
	return canonicalOnDiskThrough(checkRegularFileUnder, root)
}

// canonicalOnDiskThrough is canonicalOnDisk with the probe of the path given, so that a probe that fails in a way
// that is not a verdict about the path (it returns no refusal) can be told apart from every verdict.
func canonicalOnDiskThrough(probe func(root, repoPath string) error, root string) (Selection, error) {
	return selectionOfTheProbe(probe(root, CanonicalPath))
}

func selectionOfTheProbe(err error) (Selection, error) {
	if err == nil {
		return Selection{UncommittedCanonical: true}, nil
	}
	var refusal *ConfigError
	switch {
	case !errors.As(err, &refusal):
		return Selection{}, err
	case refusal.AbsentFromDisk():
		return Selection{}, nil
	}
	return Selection{UnusableCanonical: refusal}, nil
}
