package gitrepo

import "testing"

func TestListSnapshotEntriesRefusesAGitDirectoryWhereverTheListingHoldsIt(t *testing.T) {
	withTreeListing(t, "100644 blob 0123456789012345678901234567890123456789\tok.go\x00"+
		"100644 blob 0123456789012345678901234567890123456789\tsub/.GIT/config\x00")

	entries, err := listSnapshotEntries("", "HEAD")

	if err == nil {
		t.Fatalf("listSnapshotEntries = %+v, nil; want the tree refused whole, with the path named", entries)
	}
	if entries != nil {
		t.Errorf("entries %+v came back with the refusal: a caller that ignored the error would write them", entries)
	}
}
