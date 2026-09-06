package codesignalcli

import (
	"context"
	"os"
	"path/filepath"
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
			if got := isExactVersion(tc.value); got != tc.want {
				t.Errorf("isExactVersion(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

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
			got := parseMiseToolValue(tc.value)
			if len(got) != len(tc.want) {
				t.Fatalf("parseMiseToolValue(%q) = %v, want %v", tc.value, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseMiseToolValue(%q)[%d] = %q, want %q", tc.value, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestResolveCompilerForRuntimeDoesNotFallThroughDeclaredProjectOrigin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"typescript":"7.0.2"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"9.9.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origLocate := locateMiseTypescriptInstall
	t.Cleanup(func() { locateMiseTypescriptInstall = origLocate })
	locateMiseTypescriptInstall = func(_ context.Context, version string) (string, bool) {
		if version == "9.9.9" {
			return "/tmp/fake-mise-typescript-9.9.9", true
		}
		return "", false
	}

	got, err := resolveCompilerForRuntime(dir, nil)
	if err == nil {
		t.Fatalf("resolveCompilerForRuntime() = origin=%q version=%q path=%q, want error because the project origin declared 7.0.2 and that compiler is not on disk", got.Origin, got.Version, got.Path)
	}
}
