package gitrepo

import "fmt"

// BlobReadError is a blob, or the revision that should hold it, that could not be read. It is the cause of the
// OperationalError that BlobEntryAtRevision returns, and it says what failed in terms of the repository only:
// which revision, which path, and git's own failure. It names no command of Coach's: the layer that knows which
// command the user ran words it, and words an object a partial clone lacks as that, with the way to fetch it.
type BlobReadError struct {
	Revision string
	// Path is empty when the revision itself could not be verified.
	Path  string
	Cause error
}

func (e *BlobReadError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("revision %q could not be verified: %s", e.Revision, e.Cause)
	}
	return fmt.Sprintf("%q could not be read at revision %q: %s", e.Path, e.Revision, e.Cause)
}

func (e *BlobReadError) Unwrap() error { return e.Cause }
