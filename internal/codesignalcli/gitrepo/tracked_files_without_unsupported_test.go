package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWithoutUnsupportedFileKeepsDiscoveredEqualToItsParts(t *testing.T) {
	group := func(language string, count int) codesignal.CoverageGroup {
		return codesignal.CoverageGroup{Reason: "unsupported_language", Language: language, Count: count}
	}
	tests := []struct {
		name     string
		path     string
		in       codesignal.Coverage
		wantFile int
		want     []codesignal.CoverageGroup
	}{
		{
			name:     "the only file of an extension removes its group",
			path:     "go.mod",
			in:       codesignal.Coverage{TrackedFilesDiscovered: 3, Unsupported: []codesignal.CoverageGroup{group(".json", 1), group(".mod", 1)}},
			wantFile: 2,
			want:     []codesignal.CoverageGroup{group(".json", 1)},
		},
		{
			name:     "one of several files of an extension lowers the count",
			path:     "docs/b.json",
			in:       codesignal.Coverage{TrackedFilesDiscovered: 3, Unsupported: []codesignal.CoverageGroup{group(".json", 2)}},
			wantFile: 2,
			want:     []codesignal.CoverageGroup{group(".json", 1)},
		},
		{
			name:     "a file without an extension uses the stable label",
			path:     "Makefile",
			in:       codesignal.Coverage{TrackedFilesDiscovered: 2, Unsupported: []codesignal.CoverageGroup{group("(no extension)", 1), group(".md", 1)}},
			wantFile: 1,
			want:     []codesignal.CoverageGroup{group(".md", 1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertWithoutUnsupportedFile(t, tt.in, tt.path, tt.wantFile, tt.want)
		})
	}
}

// assertWithoutUnsupportedFile checks the three promises of
// gitrepo.WithoutUnsupportedFile for one input: the discovered count drops by
// the removed file, the unsupported groups shrink as described, and the input
// coverage is left untouched.
func assertWithoutUnsupportedFile(t *testing.T, in codesignal.Coverage, path string, wantFile int, want []codesignal.CoverageGroup) {
	t.Helper()
	original := append([]codesignal.CoverageGroup(nil), in.Unsupported...)

	got := gitrepo.WithoutUnsupportedFile(in, path)

	if got.TrackedFilesDiscovered != wantFile {
		t.Errorf("WithoutUnsupportedFile(%q) TrackedFilesDiscovered = %d, want %d", path, got.TrackedFilesDiscovered, wantFile)
	}
	if !reflect.DeepEqual(got.Unsupported, want) {
		t.Errorf("WithoutUnsupportedFile(%q) Unsupported = %+v, want %+v", path, got.Unsupported, want)
	}
	if !reflect.DeepEqual(in.Unsupported, original) {
		t.Errorf("WithoutUnsupportedFile(%q) edited the input coverage in place: %+v, was %+v", path, in.Unsupported, original)
	}
}
