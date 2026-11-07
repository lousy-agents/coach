package gitrepo

import "testing"

func TestParseSnapshotEntriesRefusesAGitDirectoryWhereverTheListingHoldsIt(t *testing.T) {
	listing := "100644 blob 0123456789012345678901234567890123456789\tok.go\x00" +
		"100644 blob 0123456789012345678901234567890123456789\tsub/.GIT/config\x00"

	entries, err := parseSnapshotEntries([]byte(listing))

	if err == nil {
		t.Fatalf("parseSnapshotEntries = %+v, nil; want the tree refused whole, with the path named", entries)
	}
	if entries != nil {
		t.Errorf("entries %+v came back with the refusal: a caller that ignored the error would write them", entries)
	}
}
