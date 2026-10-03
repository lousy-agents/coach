package codesignalcli

import (
	"os"
	"path/filepath"
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

func writeFakeCompilerInstall(t *testing.T, root, version string) string {
	t.Helper()
	compilerDir := filepath.Join(root, "node_modules", "typescript")
	if err := os.MkdirAll(compilerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(compilerDir, "package.json"), []byte(`{"name":"typescript","version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	nativeDir := nativePackageDirNextTo(compilerDir)
	if err := os.MkdirAll(nativeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(`{"name":"`+NativeTypescriptPackageName()+`","version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return compilerDir
}

func TestResolveCompilerForRuntimeRefusesUninstalledMisePin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"typescript":"`+newestSupportedTypescriptVersion()+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"9.9.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubMiseInstall(t, "9.9.9", filepath.Join(t.TempDir(), "never-installed"))

	got, err := resolveCompilerForRuntime(dir, nil)
	if err == nil {
		t.Fatalf("resolveCompilerForRuntime() = origin=%q version=%q path=%q, want an error because no origin has a compiler on disk", got.Origin, got.Version, got.Path)
	}
}
