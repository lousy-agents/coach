package gitrepo

import (
	"path/filepath"
	"testing"
)

// The files that build a bounded read are the ones the table names.

// The plumbing that BoundedRunner and RunBytesBounded are made of is not a read that names a budget.
var boundedReadPlumbing = map[string]bool{"runner.go": true, "bounded_command.go": true, "command.go": true}

func absolute(t *testing.T, path string) string {
	t.Helper()
	full, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return full
}

// buildsABoundedRead says whether the file calls BoundedRunner or RunBytesBounded to hold a read to its limits.
func buildsABoundedRead(t *testing.T, path string) bool {
	t.Helper()
	return !boundedReadPlumbing[filepath.Base(path)] && len(callsOf(parseFile(t, path), "BoundedRunner", "RunBytesBounded")) > 0
}

func filesBuildingABoundedReadUnder(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, path := range goSourceFilesUnder(t, root) {
		if buildsABoundedRead(t, path) {
			files = append(files, absolute(t, path))
		}
	}
	return files
}

// filesBuildingABoundedRead is the files of the scan, found in the source, that call BoundedRunner or
// RunBytesBounded to hold a read to its limits.
func filesBuildingABoundedRead(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range []string{"../..", "../../../cmd"} {
		files = append(files, filesBuildingABoundedReadUnder(t, root)...)
	}
	return files
}

// The table holds a row for every file that holds a read to a budget through BoundedRunner or RunBytesBounded,
// found in the source and not in a list written beside the table.
func TestEveryFileThatBuildsABoundedReadHasARowInTheTable(t *testing.T) {
	inTable := map[string]bool{}
	for _, site := range readSites {
		inTable[absolute(t, filepath.Join(site.dir, site.file))] = true
	}
	files := filesBuildingABoundedRead(t)
	if len(files) < 4 {
		t.Fatalf("found only %v: the scan of the source is not finding the files that build a bounded read", files)
	}
	for _, path := range files {
		if !inTable[path] {
			t.Errorf("%s holds a read to a budget and has no row in readSites: its limits are nobody's decision", path)
		}
	}
}
