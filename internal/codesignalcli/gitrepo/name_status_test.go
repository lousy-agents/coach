package gitrepo

import (
	"slices"
	"strings"
	"testing"
)

func TestParseNameStatusZUnusualPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "spaces", path: "a b/c d.go"},
		{name: "quotes", path: `a"b.go`},
		{name: "newline", path: "a\nb.go"},
		{name: "non-ascii", path: "café/日本語.go"},
		{name: "shell metacharacters", path: "$(rm -rf /); `echo pwned`; a&&b.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkParseNameStatusZUnusualPaths(t, tt)
		})
	}
}

func checkParseNameStatusZUnusualPaths(t *testing.T, tt struct {
	name string
	path string
}) {
	payload := joinNUL("M", tt.path)
	got, err := parseNameStatusZ(payload)
	if err != nil {
		t.Fatalf("parseNameStatusZ: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].paths[0] != tt.path {
		t.Fatalf("parseNameStatusZ(%q) = %#v, want single record with path %q", payload, got, tt.path)
	}
}

func recordsEqual(a, b []nameStatusRecord) bool {
	return slices.EqualFunc(a, b, func(x, y nameStatusRecord) bool {
		return x.status == y.status && slices.Equal(x.paths, y.paths)
	})
}

func TestParseNameStatusZ(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    []nameStatusRecord
		wantErr bool
	}{
		{
			name:    "empty",
			payload: []byte{},
			want:    nil,
		},
		{
			name:    "added",
			payload: joinNUL("A", "a.go"),
			want:    []nameStatusRecord{{status: "A", paths: []string{"a.go"}}},
		},
		{
			name:    "modified",
			payload: joinNUL("M", "a.go"),
			want:    []nameStatusRecord{{status: "M", paths: []string{"a.go"}}},
		},
		{
			name:    "deleted",
			payload: joinNUL("D", "a.go"),
			want:    []nameStatusRecord{{status: "D", paths: []string{"a.go"}}},
		},
		{
			name:    "rename with score consumes two paths",
			payload: joinNUL("R100", "old.go", "new.go"),
			want:    []nameStatusRecord{{status: "R100", paths: []string{"old.go", "new.go"}}},
		},
		{
			name:    "copy with score consumes two paths",
			payload: joinNUL("C75", "src.go", "dst.go"),
			want:    []nameStatusRecord{{status: "C75", paths: []string{"src.go", "dst.go"}}},
		},
		{
			name:    "type change other status consumes one path",
			payload: joinNUL("T", "link.go"),
			want:    []nameStatusRecord{{status: "T", paths: []string{"link.go"}}},
		},
		{
			name: "record alignment not thrown off by preceding multi-path record",
			payload: joinNUL(
				"R100", "old.go", "new.go",
				"M", "unrelated.go",
			),
			want: []nameStatusRecord{
				{status: "R100", paths: []string{"old.go", "new.go"}},
				{status: "M", paths: []string{"unrelated.go"}},
			},
		},
		{
			name:    "empty status is malformed",
			payload: joinNUL("", "a.go"),
			wantErr: true,
		},
		{
			name:    "truncated rename record is malformed",
			payload: joinNUL("R100", "old.go"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkParseNameStatusZ(t, tt)
		})
	}
}

func checkParseNameStatusZ(t *testing.T, tt struct {
	name    string
	payload []byte
	want    []nameStatusRecord
	wantErr bool
}) {
	got, err := parseNameStatusZ(tt.payload)
	if tt.wantErr {
		if err == nil {
			t.Fatalf("parseNameStatusZ(%q): want error, got nil", tt.payload)
		}
		return
	}
	if err != nil {
		t.Fatalf("parseNameStatusZ(%q): unexpected error: %v", tt.payload, err)
	}
	if !recordsEqual(got, tt.want) {
		t.Errorf("parseNameStatusZ(%q) = %#v, want %#v", tt.payload, got, tt.want)
	}
}

func joinNUL(fields ...string) []byte {
	return []byte(strings.Join(fields, "\x00") + "\x00")
}
