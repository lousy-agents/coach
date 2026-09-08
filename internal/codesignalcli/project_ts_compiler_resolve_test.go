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

func stubGlobalMise(t *testing.T, version string) {
	t.Helper()
	original := detectGlobalMiseTypescriptVersion
	t.Cleanup(func() { detectGlobalMiseTypescriptVersion = original })
	detectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return version, true
	}
}

func TestResolveCompilerForRuntimeFallsThroughAbsentProjectOriginToMise(t *testing.T) {
	dir := t.TempDir()
	supported := newestSupportedTypescriptVersion()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"typescript":"`+supported+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \""+supported+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compilerDir := writeFakeCompilerInstall(t, t.TempDir(), supported)
	stubMiseInstall(t, supported, compilerDir)

	got, err := resolveCompilerForRuntime(dir, nil)
	if err != nil {
		t.Fatalf("resolveCompilerForRuntime() error = %v, want the installed project-mise compiler", err)
	}
	if got.Origin != compilerOriginMiseProject || got.Version != supported || got.Path != compilerDir {
		t.Errorf("resolveCompilerForRuntime() = origin=%q version=%q path=%q, want origin=%q version=%q path=%q", got.Origin, got.Version, got.Path, compilerOriginMiseProject, supported, compilerDir)
	}
	if check := resolveCompiler(dir, nil); check.State != ReadinessPass || check.Version != supported {
		t.Errorf("resolveCompiler() = %+v, want a pass on the same compiler the runtime resolved", check)
	}
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

// checks.compiler carries no origin field, so this is the only level that
// can assert which origin won.
func TestEvaluateCompilerOriginsSelectsGlobalMiseAfterUnavailableProjectOrigin(t *testing.T) {
	dir := t.TempDir()
	supported := newestSupportedTypescriptVersion()
	if err := os.MkdirAll(filepath.Join(dir, "apps", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "apps", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "apps", "api", "package.json"), []byte(`{"devDependencies":{"typescript":"5.9.3"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	compilerDir := writeFakeCompilerInstall(t, t.TempDir(), supported)
	stubGlobalMise(t, supported)
	stubMiseInstall(t, supported, compilerDir)

	aggregate := evaluateCompilerOrigins(dir, []string{"apps/web", "apps/api"})
	if aggregate.winner == nil {
		t.Fatalf("evaluateCompilerOrigins() winner = nil, want the global-mise candidate; candidates=%+v", aggregate.candidates)
	}
	if aggregate.winner.origin != compilerOriginMiseGlobal {
		t.Errorf("evaluateCompilerOrigins() winner origin = %q, want %q", aggregate.winner.origin, compilerOriginMiseGlobal)
	}
	mismatches := aggregate.declarationMismatches()
	if len(mismatches) != 1 || mismatches[0].Declared != "5.9.3" || mismatches[0].Root != "apps/api" {
		t.Errorf("declarationMismatches() = %+v, want one entry {Root: apps/api, Declared: 5.9.3}", mismatches)
	}
}
