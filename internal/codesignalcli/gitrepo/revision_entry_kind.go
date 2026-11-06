package gitrepo

import (
	"fmt"
	"strings"
)

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
// repository root, as BlobEntryAtRevision does. A record for any other path, or one it cannot read, is an
// error: guessing would call a directory a file or a file absent.
func EntryKindAtRevision(run Runner, dir, revision, repoPath string) (EntryKind, error) {
	output, err := run(dir, "ls-tree", "-z", "--full-tree", revision, "--", ":(top,literal)"+repoPath)
	if err != nil {
		return EntryAbsent, &OperationalError{Message: fmt.Sprintf("coach codesignal: %q could not be resolved at revision %q: %s", repoPath, revision, err), Cause: err}
	}
	records := SplitNULPaths(output)
	if len(records) == 0 {
		return EntryAbsent, nil
	}
	kind, err := entryKindOf(records[0], repoPath)
	if err != nil || len(records) > 1 {
		return EntryAbsent, &OperationalError{Message: fmt.Sprintf("coach codesignal: %q could not be resolved at revision %q: git printed a record this version cannot read", repoPath, revision), Cause: errUnparseableIndexLine}
	}
	return kind, nil
}

// entryKindOf reads "<mode> <type> <object>\t<path>" and refuses a record for another path.
func entryKindOf(record, repoPath string) (EntryKind, error) {
	meta, path, found := strings.Cut(record, "\t")
	fields := strings.Fields(meta)
	if !found || len(fields) != 3 || !SamePath(path, repoPath) {
		return EntryAbsent, errUnparseableIndexLine
	}
	switch {
	case fields[1] == "tree":
		return EntryDirectory, nil
	case fields[1] == "commit" && fields[0] == "160000":
		return EntrySubmodule, nil
	case fields[1] == "blob" && fields[0] == "120000":
		return EntrySymbolicLink, nil
	case fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755"):
		return EntryRegularFile, nil
	}
	return EntryOther, nil
}
