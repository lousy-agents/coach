package gitrepo

import (
	"errors"
	"io/fs"
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

func TestIndexEntriesKeepsGitsOwnErrorAsTheCause(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }

	_, err := IndexEntries(run, "repo", "pol.json")

	var operational *OperationalError
	if !errors.As(err, &operational) || operational.Cause == nil || operational.Cause.Error() != "boom" {
		t.Errorf("IndexEntries error = %#v; want Cause to be git's own error, so a caller can quote it without Coach's wrapper", err)
	}
}

func TestIndexEntriesNamesTheParseFailureAsTheCauseWithoutCoachsWrapper(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return []byte("not an index line\n"), nil }

	_, err := IndexEntries(run, "repo", "pol.json")

	var operational *OperationalError
	if !errors.As(err, &operational) || operational.Cause == nil {
		t.Fatalf("IndexEntries error = %#v; want a Cause that says why the listing was refused", err)
	}
	if !strings.Contains(operational.Cause.Error(), "cannot parse") || strings.Contains(operational.Cause.Error(), "coach codesignal") {
		t.Errorf("Cause = %q; want the parse reason alone", operational.Cause)
	}
}

func TestAnOperationalErrorDoesNotLetErrorsAsReachItsCause(t *testing.T) {
	err := error(&OperationalError{Message: "m", Cause: &fs.PathError{Op: "chdir", Path: "/secret/repo", Err: errors.New("denied")}})

	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		t.Errorf("errors.As found %v inside an OperationalError; a caller that tests *fs.PathError first would take it for a different failure", pathErr)
	}
}

func TestIndexEntriesRefusesAListingLineItCannotRead(t *testing.T) {
	const blob = "0123456789012345678901234567890123456789"
	cases := []struct {
		name   string
		output string
	}{
		{"a missing field", "H 100644 " + blob + "\tpol.json\n"},
		{"an extra field", "H 100644 " + blob + " 0 x\tpol.json\n"},
		{"no tab before the path", "H 100644 " + blob + " 0 pol.json\n"},
		{"an object id that is not hexadecimal", "H 100644 zzzz" + blob[4:] + " 0\tpol.json\n"},
		{"an object id of a length no object format has", "H 100644 " + blob + "0 0\tpol.json\n"},
		{"a mode that is not six octal digits", "H 10064 " + blob + " 0\tpol.json\n"},
		{"a mode with a digit that is not octal", "H 100948 " + blob + " 0\tpol.json\n"},
		{"a stage that is not 0 to 3", "H 100644 " + blob + " 7\tpol.json\n"},
		{"a tag of more than one character", "HH 100644 " + blob + " 0\tpol.json\n"},
		{"a tag git does not print for an index entry", "X 100644 " + blob + " 0\tpol.json\n"},
		{"a tag in the wrong case", "s 100644 " + blob + " 0\tpol.json\n"},
		{"the tag of an untracked path", "? 100644 " + blob + " 0\tpol.json\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := func(string, ...string) ([]byte, error) { return []byte(c.output), nil }

			entries, err := IndexEntries(run, "repo", "pol.json")

			if err == nil {
				t.Fatalf("IndexEntries = %v, nil; want an error: a line it cannot read must not be taken for an entry", entries)
			}
		})
	}
}

func TestIndexEntriesReadsAnObjectIdOfEitherObjectFormat(t *testing.T) {
	const sha256 = "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813"
	run := func(string, ...string) ([]byte, error) { return []byte("H 100644 " + sha256 + " 0\tpol.json\n"), nil }

	entries, err := IndexEntries(run, "repo", "pol.json")

	if err != nil || len(entries) != 1 || entries[0].SHA != sha256 {
		t.Errorf("IndexEntries = %v, %v; want the one entry with the 64-digit id", entries, err)
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
			if got := IsEmptyBlob(c.id); got != c.want {
				t.Errorf("IsEmptyBlob(%q) = %v, want %v", c.id, got, c.want)
			}
		})
	}
}
