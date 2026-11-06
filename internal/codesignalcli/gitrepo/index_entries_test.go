package gitrepo

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestIndexEntries(t *testing.T) {
	const blob = "0123456789012345678901234567890123456789"
	cases := []struct {
		name   string
		output string
		want   []IndexEntry
	}{
		{"a path the index does not know", "", nil},
		{"one entry", "H 100644 " + blob + " 0\tpol.json\x00", []IndexEntry{{TreeEntry: TreeEntry{Mode: "100644", SHA: blob}, Stage: "0"}}},
		{"a path the sparse checkout skips", "S 100644 " + blob + " 0\tpol.json\x00", []IndexEntry{{TreeEntry: TreeEntry{Mode: "100644", SHA: blob}, Stage: "0", SkipWorktree: true}}},
		{"a merge conflict", "M 100644 " + blob + " 1\tpol.json\x00M 100644 " + blob + " 2\tpol.json\x00", []IndexEntry{
			{TreeEntry: TreeEntry{Mode: "100644", SHA: blob}, Stage: "1"},
			{TreeEntry: TreeEntry{Mode: "100644", SHA: blob}, Stage: "2"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertIndexEntries(t, c.output, c.want)
		})
	}
}

func assertIndexEntries(t *testing.T, output string, want []IndexEntry) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return []byte(output), nil }

	got, err := IndexEntries(run, "repo", "pol.json")

	if err != nil || !reflect.DeepEqual(got.Entries, want) || got.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want entries %v and nothing below the path", got, err, want)
	}
}

func TestIndexEntriesMatchesThePathRelativeToTheRepositoryRootLiterally(t *testing.T) {
	var gotArgs []string
	run := func(_ string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	if _, err := IndexEntries(run, "repo", "a[1]*.json"); err != nil {
		t.Fatalf("IndexEntries: %v", err)
	}

	want := []string{"ls-files", "--stage", "-t", "-z", "--full-name", "--", ":(top,literal)a[1]*.json"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("git args = %v; want %v", gotArgs, want)
	}
}

func TestIndexEntriesFailsClosedWhenGitFails(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }

	_, err := IndexEntries(run, "repo", "pol.json")

	var operational *OperationalError
	if !errors.As(err, &operational) || !strings.Contains(err.Error(), "pol.json") {
		t.Errorf("IndexEntries error = %v; want an *OperationalError naming the path", err)
	}
}

func TestIndexEntriesKeepsGitsOwnErrorAsTheCause(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }

	_, err := IndexEntries(run, "repo", "pol.json")

	var operational *OperationalError
	if !errors.As(err, &operational) || operational.Cause == nil || operational.Cause.Error() != "boom" {
		t.Errorf("IndexEntries error = %#v; want Cause to be git's own error, so a caller can quote it without Coach's wrapper", err)
	}
}

func TestIndexEntriesReadsAnObjectIdOfEitherObjectFormat(t *testing.T) {
	const sha256 = "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813"
	run := func(string, ...string) ([]byte, error) { return []byte("H 100644 " + sha256 + " 0\tpol.json\x00"), nil }

	entries, err := IndexEntries(run, "repo", "pol.json")

	if err != nil || len(entries.Entries) != 1 || entries.Entries[0].SHA != sha256 {
		t.Errorf("IndexEntries = %+v, %v; want the one entry with the 64-digit id", entries, err)
	}
}

func TestIsEmptyBlobKnowsTheEmptyBlobOfEveryObjectFormat(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want bool
	}{
		{"the SHA-1 empty blob", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", true},
		{"the SHA-256 empty blob", "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813", true},
		{"another SHA-1 blob", "0123456789012345678901234567890123456789", false},
		{"another SHA-256 blob", "0123456789012345678901234567890123456789012345678901234567890123", false},
		{"the SHA-1 empty blob's digits at the SHA-256 length", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", false},
		{"nothing", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertIsEmptyBlob(t, c.id, c.want)
		})
	}
}

func assertIsEmptyBlob(t *testing.T, id string, want bool) {
	t.Helper()
	if got := IsEmptyBlob(id); got != want {
		t.Errorf("IsEmptyBlob(%q) = %v, want %v", id, got, want)
	}
}
