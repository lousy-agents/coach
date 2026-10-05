package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestParseChangedRanges(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		want    []codesignal.LineRange
		wantErr bool
	}{
		{
			name: "first-line insertion",
			diff: "diff --git a/f.go b/f.go\n" +
				"--- a/f.go\n" +
				"+++ b/f.go\n" +
				"@@ -0,0 +1,3 @@\n" +
				"+a\n+b\n+c\n",
			want: []codesignal.LineRange{{StartRow: 0, EndRow: 2}},
		},
		{
			name: "multi-line replacement",
			diff: "@@ -2,2 +2,4 @@\n" +
				"-old1\n-old2\n+new1\n+new2\n+new3\n+new4\n",
			want: []codesignal.LineRange{{StartRow: 1, EndRow: 4}},
		},
		{
			name: "deletion-only hunk emits no range",
			diff: "@@ -5,3 +4,0 @@\n" +
				"-a\n-b\n-c\n",
			want: nil,
		},
		{
			name: "two disjoint hunks",
			diff: "@@ -1,1 +1,1 @@\n" +
				"-a\n+z\n" +
				"@@ -10,1 +10,2 @@\n" +
				"-j\n+k\n+l\n",
			want: []codesignal.LineRange{
				{StartRow: 0, EndRow: 0},
				{StartRow: 9, EndRow: 10},
			},
		},
		{
			name: "eof-adjacent addition",
			diff: "@@ -20,0 +21,2 @@\n" +
				"+x\n+y\n",
			want: []codesignal.LineRange{{StartRow: 20, EndRow: 21}},
		},
		{
			name: "omitted count means 1",
			diff: "@@ -1 +1 @@\n" +
				"-a\n+b\n",
			want: []codesignal.LineRange{{StartRow: 0, EndRow: 0}},
		},
		{
			name: "no hunks",
			diff: "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n",
			want: nil,
		},
		{
			name:    "malformed hunk header",
			diff:    "@@ garbage @@\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_analyzePart4Test_123(t, tt)
		})
	}
}

func body_analyzePart4Test_123(t *testing.T, tt struct {
	name    string
	diff    string
	want    []codesignal.LineRange
	wantErr bool
}) {
	got, err := parseChangedRanges([]byte(tt.diff))
	if tt.wantErr {
		if err == nil {
			t.Fatalf("parseChangedRanges(%q): want error, got nil", tt.diff)
		}
		return
	}
	if err != nil {
		t.Fatalf("parseChangedRanges(%q): unexpected error: %v", tt.diff, err)
	}
	if !rangesEqual(got, tt.want) {
		t.Errorf("parseChangedRanges(%q) = %#v, want %#v", tt.diff, got, tt.want)
	}
}

func rangesEqual(a, b []codesignal.LineRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
