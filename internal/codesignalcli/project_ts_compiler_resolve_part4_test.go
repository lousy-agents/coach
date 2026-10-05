package codesignalcli

import (
	"context"

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
			body_projectTsCompilerResolvePart4Test_28(t, tc)
		})
	}
}

func stubMiseInstall(t *testing.T, version, compilerDir string) {
	t.Helper()
	original := locateMiseTypescriptInstall
	t.Cleanup(func() { locateMiseTypescriptInstall = original })
	locateMiseTypescriptInstall = func(_ context.Context, requested string) (string, bool) {
		if requested == version {
			return compilerDir, true
		}
		return "", false
	}
}
