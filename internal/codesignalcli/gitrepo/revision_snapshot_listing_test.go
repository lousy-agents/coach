package gitrepo

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// listingFailing is a lister whose tree cannot be listed.
func listingFailing(failure error) snapshotLister {
	return func(string, string) ([]snapshotEntry, error) { return nil, failure }
}

// The snapshot is the tree, so a tree that cannot be listed, or that is listed in words Coach cannot read, is an
// error: a scan of an empty snapshot would report no findings for a repository it never read.
func TestExtractRevisionIsAnErrorWhenTheTreeCannotBeListed(t *testing.T) {
	boom := errors.New("fatal: simulated ls-tree failure")
	overrun := &BoundError{Kind: BoundTimeout, message: "git execution timed out after 2m0s"}
	cases := []struct {
		name    string
		failure error
	}{
		{"git fails", boom},
		{"git does not answer in time", overrun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertSnapshotRefused(t, listingFailing(c.failure), c.failure)
		})
	}
}

// And so is a stream whose records Coach cannot read, through the real lister.
func TestExtractRevisionIsAnErrorWhenTheListingCannotBeRead(t *testing.T) {
	cases := []struct{ name, listing string }{
		{"a record has no path", "100644 blob 0123456789012345678901234567890123456789\x00"},
		{"a record has too few fields", "100644 blob\tpath\x00"},
		{"the stream ends inside a record", "100644 blob 0123456789012345678901234567890123456789\tpath"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withTreeListing(t, c.listing)
			assertSnapshotRefused(t, listSnapshotEntries, nil)
		})
	}
}

func assertSnapshotRefused(t *testing.T, list snapshotLister, wantErr error) {
	t.Helper()
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)

	dir, err := extractRevision(list, func(string, ...string) ([]byte, error) { return nil, errors.New("not asked") }, "repository", "HEAD")

	if err == nil {
		t.Fatalf("extractRevision = %q, nil; want the listing's failure returned and no snapshot", dir)
	}
	if !strings.Contains(err.Error(), `reading source snapshot "HEAD"`) {
		t.Errorf("error %q does not say which read failed", err)
	}
	if wantErr != nil && !errors.Is(err, wantErr) {
		t.Errorf("error %v does not carry %v, so a caller cannot tell a bound from a failure", err, wantErr)
	}
	if left, _ := os.ReadDir(temporary); len(left) != 0 {
		t.Errorf("extractRevision left %d entries in the temporary directory after a listing it could not read", len(left))
	}
}
