package codesignalcli

import (
	"testing"
)

func recordsEqual(a, b []nameStatusRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].status != b[i].status {
			return false
		}
		if len(a[i].paths) != len(b[i].paths) {
			return false
		}
		for j := range a[i].paths {
			if a[i].paths[j] != b[i].paths[j] {
				return false
			}
		}
	}
	return true
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
			body_gitPart5Test_93(t, tt)
		})
	}
}
