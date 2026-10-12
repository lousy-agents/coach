package tstoolchain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

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
