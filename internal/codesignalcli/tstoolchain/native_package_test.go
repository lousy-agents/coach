package tstoolchain

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveNativePackageFallsBackToRealDirectoryBesideSymlink mirrors real
// mise 2026.9.5's layout: mise's default npm backend hoists the compiler
// itself to a symlink but not its native optionalDependency sibling, which
// is only ever installed beside the real, symlink-resolved directory.
// resolveNativePackage must find it there by reading the filesystem
// (filepath.EvalSymlinks), never by mutating it.
func TestResolveNativePackageFallsBackToRealDirectoryBesideSymlink(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, ".mise", "typescript@7.0.2", "node_modules", "typescript")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "package.json"), []byte(`{"name":"typescript","version":"7.0.2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	realNativeDir := nativePackageDirNextTo(realDir)
	if err := os.MkdirAll(realNativeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realNativeDir, "package.json"), []byte(`{"name":"`+NativeTypescriptPackageName()+`","version":"7.0.2"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	symlinkParent := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(symlinkParent, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(symlinkParent, "typescript")
	if err := os.Symlink(realDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	dir, version, ok := resolveNativePackage(symlinkPath, "7.0.2")
	if !ok {
		t.Fatalf("resolveNativePackage(%q, ...) ok = false, want true", symlinkPath)
	}
	if version != "7.0.2" {
		t.Fatalf("version = %q, want 7.0.2", version)
	}
	wantDir, err := filepath.EvalSymlinks(realNativeDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", realNativeDir, err)
	}
	if dir != wantDir {
		t.Fatalf("dir = %q, want the native package beside the real, symlink-resolved directory: %q", dir, wantDir)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("dir = %q, want an absolute path -- this is exactly what the analyzer receives as --native-package=", dir)
	}
}
