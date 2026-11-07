package tstoolchain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

// miseInstalling is a locator that finds the compiler of version in compilerDir, as if mise had installed it there, and
// nothing for any other version.
func miseInstalling(version, compilerDir string) MiseLocator {
	return func(_ context.Context, requested string) (string, bool) {
		if requested == version {
			return compilerDir, true
		}
		return "", false
	}
}
