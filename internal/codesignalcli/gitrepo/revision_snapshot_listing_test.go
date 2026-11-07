package gitrepo

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// listingOnly is all of git for a snapshot whose listing is the only thing it is asked: it answers the full
// listing of the tree with what it is given and fails the test on any other question, so the error under test is
// the one the listing made and nothing else ran.
func listingOnly(t *testing.T, output string, failure error) Runner {
	t.Helper()
	return func(_ string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "ls-tree -r -z --full-tree HEAD" {
			return []byte(output), failure
		}
		t.Errorf("git was asked %q; the fake answers only the listing", strings.Join(args, " "))
		return nil, errors.New("not asked")
	}
}

// The snapshot is the tree, so a tree that cannot be listed, or that is listed in words Coach cannot read, is an
// error: a scan of an empty snapshot would report no findings for a repository it never read.
func TestExtractRevisionIsAnErrorWhenTheTreeCannotBeListedOrTheListingCannotBeRead(t *testing.T) {
	boom := errors.New("fatal: simulated ls-tree failure")
	overrun := &BoundError{Kind: BoundStdout, message: "git stdout exceeded 67108864-byte budget"}
	cases := []struct {
		name    string
		run     func(*testing.T) Runner
		wantErr error
	}{
		{"git fails", func(t *testing.T) Runner { return listingOnly(t, "", boom) }, boom},
		{"git prints more than the listing may hold", func(t *testing.T) Runner { return listingOnly(t, "", overrun) }, overrun},
		{"a record has no path", func(t *testing.T) Runner {
			return listingOnly(t, "100644 blob 0123456789012345678901234567890123456789\x00", nil)
		}, nil},
		{"a record has too few fields", func(t *testing.T) Runner { return listingOnly(t, "100644 blob\tpath\x00", nil) }, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertSnapshotRefused(t, c.run(t), c.wantErr)
		})
	}
}

func assertSnapshotRefused(t *testing.T, run Runner, wantErr error) {
	t.Helper()
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)

	dir, err := extractRevision(run, "repository", "HEAD")

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
