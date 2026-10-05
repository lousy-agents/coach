package tstoolchain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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

func stubGlobalMise(t *testing.T, version string) {
	t.Helper()
	original := DetectGlobalMiseTypescriptVersion
	t.Cleanup(func() { DetectGlobalMiseTypescriptVersion = original })
	DetectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return version, true
	}
}
