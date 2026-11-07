package gitrepo

import (
	"strings"
	"testing"
)

// The files an analyzer reads are written before the others.

func TestTheSnapshotReadsTheFilesAnalyzersReadFirst(t *testing.T) {
	entries := []snapshotEntry{{path: "x.png"}, {path: "pkg/a.go"}, {path: "go.mod"}, {path: "y.bin"}, {path: "tsconfig.base.json"}, {path: "web/app.TSX"}}

	var got []string
	for _, entry := range neededFirst(entries) {
		got = append(got, entry.path)
	}

	want := "pkg/a.go go.mod tsconfig.base.json web/app.TSX x.png y.bin"
	if strings.Join(got, " ") != want {
		t.Errorf("neededFirst = %v, want %q: the files an analyzer reads, in listing order, then the rest", got, want)
	}
}
