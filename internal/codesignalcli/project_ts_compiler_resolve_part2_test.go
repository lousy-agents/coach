package codesignalcli

import (
	"os"
	"path/filepath"
	"testing"
)

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
