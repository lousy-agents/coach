package gitrepo

import (
	"bufio"
	"strings"
	"testing"
)

// What git prints is parsed by Coach, and what it cannot parse is an error, never a panic and never an empty
// or missing object. Each case below is output that a git which did not do as asked could print.

type malformedListing struct{ name, record string }

func (c malformedListing) run(t *testing.T) {
	entries, err := parseSnapshotEntries([]byte(c.record + "\x00"))
	if err == nil || len(entries) != 0 {
		t.Errorf("parseSnapshotEntries(%q) = %v, %v; want an error and no entries", c.record, entries, err)
	}
}

func TestParseSnapshotEntriesRefusesWhatIsNotAnLsTreeRecord(t *testing.T) {
	cases := []malformedListing{
		{"no tab", "100644 blob abc"},
		{"only a tab", "\t"},
		{"two fields", "100644 blob\tpath"},
		{"four fields", "100644 blob abc def\tpath"},
		{"an empty path", "100644 blob abc\t"},
		{"no fields before the tab", "\tpath"},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestParseSnapshotEntriesReadsARecordAndSkipsTheEmptyOnesBetween(t *testing.T) {
	entries, err := parseSnapshotEntries([]byte("100644 blob abc\ta b.go\x00\x00100755 blob def\tbin/run\x00"))

	if err != nil || len(entries) != 2 || entries[0].path != "a b.go" || entries[0].oid != "abc" || entries[1].mode != "100755" {
		t.Errorf("parseSnapshotEntries = %v, %v; want the two records, with the path that holds a space", entries, err)
	}
}

type malformedResponse struct{ name, response string }

func (c malformedResponse) run(t *testing.T) {
	response, err := readObjectResponse(bufio.NewReader(strings.NewReader(c.response)), "abc", 1<<20)
	if err == nil || response.found || len(response.content) != 0 {
		t.Errorf("readObjectResponse(%q) = %+v, %v; want an error, and not an object that is found or missing", c.response, response, err)
	}
}

func TestReadObjectResponseRefusesWhatIsNotACatFileResponse(t *testing.T) {
	cases := []malformedResponse{
		{"nothing", ""},
		{"a header without its newline", "abc blob 5"},
		{"a negative size", "abc blob -1\n"},
		{"a size that is not a number", "abc blob x\n"},
		{"a size past an int64", "abc blob 99999999999999999999\n"},
		{"two fields that are not missing", "abc blob\n"},
		{"four fields", "abc blob 5 extra\n"},
		{"content shorter than its size", "abc blob 5\nhi"},
		{"no newline after the content", "abc blob 2\nhi"},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestReadObjectResponseReadsMissingAndFoundAndEmpty(t *testing.T) {
	missing, missingErr := readObjectResponse(bufio.NewReader(strings.NewReader("abc missing\n")), "abc", 1<<20)
	found, foundErr := readObjectResponse(bufio.NewReader(strings.NewReader("abc blob 2\nhi\n")), "abc", 1<<20)
	empty, emptyErr := readObjectResponse(bufio.NewReader(strings.NewReader("abc blob 0\n\n")), "abc", 1<<20)

	if missingErr != nil || missing.found {
		t.Errorf("missing = %+v, %v; want not found", missing, missingErr)
	}
	if foundErr != nil || !found.found || string(found.content) != "hi" {
		t.Errorf("found = %+v, %v; want hi", found, foundErr)
	}
	if emptyErr != nil || !empty.found || len(empty.content) != 0 {
		t.Errorf("empty = %+v, %v; want an empty object that is found", empty, emptyErr)
	}
}
