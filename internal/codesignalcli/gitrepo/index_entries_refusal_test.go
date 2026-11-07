package gitrepo

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestIndexEntriesFailsClosedOnALineItCannotParse(t *testing.T) {
	cases := []struct {
		name   string
		output string
	}{
		{"a line without the tab before the path", "H 100644 0123456789012345678901234567890123456789 0 pol.json\x00"},
		{"a line with too few fields", "H 100644 0\tpol.json\x00"},
		{"a line without the status tag", "100644 0123456789012345678901234567890123456789 0\tpol.json\x00"},
		{"a good line followed by a bad one", "H 100644 0123456789012345678901234567890123456789 0\tpol.json\x00not an index line\x00"},
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
	if !errors.As(err, &operational) || entries.Entries != nil || entries.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want nothing and an *OperationalError, so a listing that cannot be read is never taken for a clean index", entries, err)
	}
}

func TestIndexEntriesNamesTheParseFailureAsTheCauseWithoutCoachsWrapper(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return []byte("not an index line\x00"), nil }

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
		{"a missing field", "H 100644 " + blob + "\tpol.json\x00"},
		{"an extra field", "H 100644 " + blob + " 0 x\tpol.json\x00"},
		{"no tab before the path", "H 100644 " + blob + " 0 pol.json\x00"},
		{"an object id that is not hexadecimal", "H 100644 zzzz" + blob[4:] + " 0\tpol.json\x00"},
		{"an object id of a length no object format has", "H 100644 " + blob + "0 0\tpol.json\x00"},
		{"a mode that is not six octal digits", "H 10064 " + blob + " 0\tpol.json\x00"},
		{"a mode with a digit that is not octal", "H 100948 " + blob + " 0\tpol.json\x00"},
		{"a stage that is not 0 to 3", "H 100644 " + blob + " 7\tpol.json\x00"},
		{"a tag of more than one character", "HH 100644 " + blob + " 0\tpol.json\x00"},
		{"a tag git does not print for an index entry", "X 100644 " + blob + " 0\tpol.json\x00"},
		{"a tag in the wrong case", "s 100644 " + blob + " 0\tpol.json\x00"},
		{"the tag of an untracked path", "? 100644 " + blob + " 0\tpol.json\x00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertListingLineRefused(t, c.output)
		})
	}
}

func assertListingLineRefused(t *testing.T, output string) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return []byte(output), nil }

	entries, err := IndexEntries(run, "repo", "pol.json")

	if err == nil {
		t.Fatalf("IndexEntries = %+v, nil; want an error: a line it cannot read must not be taken for an entry", entries)
	}
}
