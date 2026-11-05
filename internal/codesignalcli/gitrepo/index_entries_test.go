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
		{"one entry", "H 100644 " + blob + " 0\tpol.json\n", []IndexEntry{{TreeEntry: TreeEntry{Mode: "100644", SHA: blob}, Stage: "0"}}},
		{"a path the sparse checkout skips", "S 100644 " + blob + " 0\tpol.json\n", []IndexEntry{{TreeEntry: TreeEntry{Mode: "100644", SHA: blob}, Stage: "0", SkipWorktree: true}}},
		{"a merge conflict", "M 100644 " + blob + " 1\tpol.json\nM 100644 " + blob + " 2\tpol.json\n", []IndexEntry{
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

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("IndexEntries = %v, %v; want %v", got, err, want)
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

	want := []string{"ls-files", "--stage", "-t", "--", ":(top,literal)a[1]*.json"}
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

func TestIndexEntriesFailsClosedOnALineItCannotParse(t *testing.T) {
	cases := []struct {
		name   string
		output string
	}{
		{"a line without the tab before the path", "H 100644 0123456789012345678901234567890123456789 0 pol.json\n"},
		{"a line with too few fields", "H 100644 0\tpol.json\n"},
		{"a line without the status tag", "100644 0123456789012345678901234567890123456789 0\tpol.json\n"},
		{"a good line followed by a bad one", "H 100644 0123456789012345678901234567890123456789 0\tpol.json\nnot an index line\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertIndexEntriesRejects(t, c.output)
		})
	}
}

func assertIndexEntriesRejects(t *testing.T, output string) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return []byte(output), nil }

	entries, err := IndexEntries(run, "repo", "pol.json")

	var operational *OperationalError
	if !errors.As(err, &operational) || entries != nil {
		t.Errorf("IndexEntries = %v, %v; want no entries and an *OperationalError, so a listing that cannot be read is never taken for a clean index", entries, err)
	}
}
