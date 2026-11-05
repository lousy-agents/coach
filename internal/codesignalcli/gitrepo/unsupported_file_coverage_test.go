package gitrepo_test

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWithUnsupportedFileKeepsDiscoveredEqualToItsParts(t *testing.T) {
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
			name:     "a new extension adds a group in extension order",
			path:     "docs/notes.md",
			in:       codesignal.Coverage{TrackedFilesDiscovered: 2, Unsupported: []codesignal.CoverageGroup{group(".json", 1), group(".mod", 1)}},
			wantFile: 3,
			want:     []codesignal.CoverageGroup{group(".json", 1), group(".md", 1), group(".mod", 1)},
		},
		{
			name:     "an extension already counted raises its count",
			path:     "extra.json",
			in:       codesignal.Coverage{TrackedFilesDiscovered: 2, Unsupported: []codesignal.CoverageGroup{group(".json", 1)}},
			wantFile: 3,
			want:     []codesignal.CoverageGroup{group(".json", 2)},
		},
		{
			name:     "a file without an extension uses the stable label",
			path:     "Makefile",
			in:       codesignal.Coverage{TrackedFilesDiscovered: 1, Unsupported: []codesignal.CoverageGroup{group(".md", 1)}},
			wantFile: 2,
			want:     []codesignal.CoverageGroup{group("(no extension)", 1), group(".md", 1)},
		},
		{
			name:     "an empty coverage gains its first group",
			path:     "a.sh",
			in:       codesignal.Coverage{},
			wantFile: 1,
			want:     []codesignal.CoverageGroup{group(".sh", 1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertWithUnsupportedFile(t, tt.in, tt.path, tt.wantFile, tt.want)
		})
	}
}

func assertWithUnsupportedFile(t *testing.T, in codesignal.Coverage, path string, wantFile int, want []codesignal.CoverageGroup) {
	t.Helper()
	original := append([]codesignal.CoverageGroup(nil), in.Unsupported...)

	got := gitrepo.WithUnsupportedFile(in, path)

	if got.TrackedFilesDiscovered != wantFile {
		t.Errorf("WithUnsupportedFile(%q) TrackedFilesDiscovered = %d, want %d", path, got.TrackedFilesDiscovered, wantFile)
	}
	if !reflect.DeepEqual(got.Unsupported, want) {
		t.Errorf("WithUnsupportedFile(%q) Unsupported = %+v, want %+v", path, got.Unsupported, want)
	}
	if !reflect.DeepEqual(in.Unsupported, original) {
		t.Errorf("WithUnsupportedFile(%q) edited the input coverage in place: %+v, was %+v", path, in.Unsupported, original)
	}
}
