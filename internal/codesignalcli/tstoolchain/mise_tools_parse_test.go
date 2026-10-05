package tstoolchain

import (
	"testing"
)

func TestParseMiseToolValue(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  []string
	}{
		{"single double-quoted", `"5.3.3"`, []string{"5.3.3"}},
		{"single single-quoted", `'5.3.3'`, []string{"5.3.3"}},
		{"array of one", `["5.3.3"]`, []string{"5.3.3"}},
		{"array of two", `["5.3.3", "5.4.0"]`, []string{"5.3.3", "5.4.0"}},
		{"mixed quote styles in array", `["5.3.3", '5.4.0']`, []string{"5.3.3", "5.4.0"}},
		{"unquoted scalar is not a candidate", `5.3.3`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectTsCompilerResolvePart3Test_23(t, tc)
		})
	}
}

func body_projectTsCompilerResolvePart3Test_23(t *testing.T, tc struct {
	name  string
	value string
	want  []string
}) {
	got := parseMiseToolValue(tc.value)
	if len(got) != len(tc.want) {
		t.Fatalf("parseMiseToolValue(%q) = %v, want %v", tc.value, got, tc.want)
	}
	for i := range got {
		if got[i] != tc.want[i] {
			t.Errorf("parseMiseToolValue(%q)[%d] = %q, want %q", tc.value, i, got[i], tc.want[i])
		}
	}
}
