package tstoolchain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// checks.compiler carries no origin field, so this is the only level that
// can assert which origin won.
func TestEvaluateCompilerOriginsSelectsGlobalMiseAfterUnavailableProjectOrigin(t *testing.T) {
	dir := t.TempDir()
	supported := NewestSupportedTypescriptVersion()
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

	aggregate := EvaluateOrigins(dir, []string{"apps/web", "apps/api"})
	if aggregate.Winner == nil {
		t.Fatalf("evaluateCompilerOrigins() winner = nil, want the global-mise candidate; candidates=%+v", aggregate.candidates)
	}
	if aggregate.Winner.Origin != OriginMiseGlobal {
		t.Errorf("evaluateCompilerOrigins() winner origin = %q, want %q", aggregate.Winner.Origin, OriginMiseGlobal)
	}
	mismatches := aggregate.declarationMismatches()
	if len(mismatches) != 1 || mismatches[0].Declared != "5.9.3" || mismatches[0].Root != "apps/api" {
		t.Errorf("declarationMismatches() = %+v, want one entry {Root: apps/api, Declared: 5.9.3}", mismatches)
	}
}

func TestResolveCompilerForRuntimeFallsThroughAbsentProjectOriginToMise(t *testing.T) {
	dir := t.TempDir()
	supported := NewestSupportedTypescriptVersion()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"typescript":"`+supported+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \""+supported+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compilerDir := writeFakeCompilerInstall(t, t.TempDir(), supported)
	stubMiseInstall(t, supported, compilerDir)

	got, err := ResolveCompilerForRuntime(dir, nil)
	if err != nil {
		t.Fatalf("resolveCompilerForRuntime() error = %v, want the installed project-mise compiler", err)
	}
	if got.Origin != OriginMiseProject || got.Version != supported || got.Path != compilerDir {
		t.Errorf("resolveCompilerForRuntime() = origin=%q version=%q path=%q, want origin=%q version=%q path=%q", got.Origin, got.Version, got.Path, OriginMiseProject, supported, compilerDir)
	}
	if check := ResolveCompiler(dir, nil); check.State != projectreadiness.Pass || check.Version != supported {
		t.Errorf("resolveCompiler() = %+v, want a pass on the same compiler the runtime resolved", check)
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
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"typescript":"`+NewestSupportedTypescriptVersion()+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"9.9.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubMiseInstall(t, "9.9.9", filepath.Join(t.TempDir(), "never-installed"))

	got, err := ResolveCompilerForRuntime(dir, nil)
	if err == nil {
		t.Fatalf("resolveCompilerForRuntime() = origin=%q version=%q path=%q, want an error because no origin has a compiler on disk", got.Origin, got.Version, got.Path)
	}
}

func stubMiseInstall(t *testing.T, version, compilerDir string) {
	t.Helper()
	original := LocateMiseTypescriptInstall
	t.Cleanup(func() { LocateMiseTypescriptInstall = original })
	LocateMiseTypescriptInstall = func(_ context.Context, requested string) (string, bool) {
		if requested == version {
			return compilerDir, true
		}
		return "", false
	}
}

func stubGlobalMise(t *testing.T, version string) {
	t.Helper()
	original := DetectGlobalMiseTypescriptVersion
	t.Cleanup(func() { DetectGlobalMiseTypescriptVersion = original })
	DetectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return version, true
	}
}
