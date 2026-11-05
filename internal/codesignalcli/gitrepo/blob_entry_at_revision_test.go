package gitrepo

import (
	"reflect"
	"testing"
)

func TestBlobEntryAtRevisionMatchesThePathRelativeToTheRepositoryRootLiterally(t *testing.T) {
	var lsTreeArgs []string
	run := func(_ string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" {
			lsTreeArgs = args
		}
		return nil, nil
	}

	if _, _, err := BlobEntryAtRevision(run, "repo", "HEAD", ":/main.go"); err != nil {
		t.Fatalf("BlobEntryAtRevision: %v", err)
	}

	want := []string{"ls-tree", "--full-tree", "HEAD", "--", ":(top,literal):/main.go"}
	if !reflect.DeepEqual(lsTreeArgs, want) {
		t.Errorf("ls-tree args = %v; want %v, so a path that reads as pathspec magic names a file of that spelling and nothing else", lsTreeArgs, want)
	}
}

func TestBlobEntryAtRevisionReportsTheModeAndObjectName(t *testing.T) {
	const blob = "0123456789012345678901234567890123456789"
	run := func(_ string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" {
			return []byte("120000 blob " + blob + "\tlink.json\n"), nil
		}
		return nil, nil
	}

	entry, found, err := BlobEntryAtRevision(run, "repo", "HEAD", "link.json")

	if err != nil || !found || entry != (TreeEntry{Mode: "120000", SHA: blob}) {
		t.Errorf("BlobEntryAtRevision = %v, %v, %v; want the symlink mode and %s", entry, found, err, blob)
	}
}
