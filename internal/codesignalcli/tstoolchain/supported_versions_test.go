package tstoolchain

import (
	"testing"
)

func TestIsExactVersion(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"plain semver", "7.0.2", true},
		{"prerelease", "7.0.2-beta.1", true},
		{"build metadata", "7.0.2+abc123", true},
		{"caret range", "^7.0.2", false},
		{"tilde range", "~7.0.2", false},
		{"wildcard", "7.0.x", false},
		{"asterisk", "*", false},
		{"tag", "latest", false},
		{"workspace protocol", "workspace:*", false},
		{"comparator range", ">=7.0.0", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkIsExactVersion(t, tc)
		})
	}
}

func checkIsExactVersion(t *testing.T, tc struct {
	name  string
	value string
	want  bool
}) {
	if got := IsExactVersion(tc.value); got != tc.want {
		t.Errorf("isExactVersion(%q) = %v, want %v", tc.value, got, tc.want)
	}
}
