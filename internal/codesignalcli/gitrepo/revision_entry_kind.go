package gitrepo

import "fmt"

// EntryKind is what a revision holds at a path.
type EntryKind int

const (
	// EntryAbsent is nothing at the path.
	EntryAbsent EntryKind = iota
	// EntryRegularFile is a blob with a regular file mode (100644 or 100755).
	EntryRegularFile
	// EntrySymbolicLink is a blob with the link mode (120000).
	EntrySymbolicLink
	// EntryDirectory is a tree.
	EntryDirectory
	// EntrySubmodule is a gitlink (160000).
	EntrySubmodule
	// EntryOther is anything else git records.
	EntryOther
)

// EntryKindAtRevision is what revision holds at repoPath, from the mode and type git records for it and
// without reading its content, through the caller's bounded runner. The path is matched literally from the
// repository root, as BlobEntryAtRevision does, and through the same parser. A record for any other path, or one
// it cannot read, is an error: guessing would call a directory a file or a file absent.
func EntryKindAtRevision(run Runner, dir, revision, repoPath string) (EntryKind, error) {
	output, err := run(dir, "ls-tree", "-z", "--full-tree", revision, "--", ":(top,literal)"+repoPath)
	if err != nil {
		return EntryAbsent, &OperationalError{Message: fmt.Sprintf("coach codesignal: %q could not be resolved at revision %q: %s", repoPath, revision, err), Cause: err}
	}
	record, found, err := readLsTreeRecord(output, repoPath)
	if err != nil {
		return EntryAbsent, &OperationalError{Message: fmt.Sprintf("coach codesignal: %q could not be resolved at revision %q: git printed a record this version cannot read", repoPath, revision), Cause: err}
	}
	if !found {
		return EntryAbsent, nil
	}
	return kindOfRecord(record), nil
}

// kindOfRecord is the kind of entry the mode and type of a record say.
func kindOfRecord(record lsTreeRecord) EntryKind {
	switch {
	case record.objectType == "tree":
		return EntryDirectory
	case record.objectType == "commit" && record.mode == "160000":
		return EntrySubmodule
	case record.objectType == "blob" && record.mode == "120000":
		return EntrySymbolicLink
	case record.objectType == "blob" && (record.mode == "100644" || record.mode == "100755"):
		return EntryRegularFile
	}
	return EntryOther
}
