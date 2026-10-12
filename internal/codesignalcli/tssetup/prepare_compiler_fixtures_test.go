package tssetup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func writeFakeInstalledTypescriptForTest(t *testing.T, installDir, version string) {
	t.Helper()
	pkgDir := filepath.Join(installDir, "node_modules", "typescript")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"typescript","version":"`+version+`"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	nativeDir := filepath.Join(installDir, "node_modules", "@typescript", tstoolchain.NativeTypescriptUnscopedName())
	if err := os.MkdirAll(nativeDir, 0o755); err != nil {
		t.Fatalf("mkdir native: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(`{"name":"`+tstoolchain.NativeTypescriptPackageName()+`","version":"`+version+`"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write native package.json: %v", err)
	}
}

// writeFailingInstallStubMiseOnPath puts a `mise` executable on t's PATH
// whose `install` subcommand always exits 1 without moving anything into
// place, modeling a genuine `mise install` failure rather than a
// declined/cancelled selection.
func writeFailingInstallStubMiseOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n" +
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n" +
		"if [ \"$1\" = \"install\" ]; then exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755); err != nil {
		t.Fatalf("write failing-install stub mise: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// writeStatefulStubMiseOnPath puts a `mise` executable on t's PATH whose
// `install` subcommand moves a pre-staged installed-typescript fixture into
// installDir, and exits 0 for everything else, mirroring the equivalent
// cmd/coach acceptance fixture: runMiseInstallInsulated's `mise install`
// call always really executes (it is not one of this package's overridable
// probe vars), so a deterministic, offline proof of the pre/post-install
// state transition needs a real (if fake) `mise` on PATH for that one call.
func writeStatefulStubMiseOnPath(t *testing.T, installDir, version string) {
	t.Helper()
	stagingDir := t.TempDir()
	writeFakeInstalledTypescriptForTest(t, stagingDir, version)

	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n" +
		"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n" +
		"if [ \"$1\" = \"install\" ]; then rmdir " + shellQuoteForTest(installDir) + " 2>/dev/null; mv " + shellQuoteForTest(stagingDir) + " " + shellQuoteForTest(installDir) + "; exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755); err != nil {
		t.Fatalf("write stateful stub mise: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// locateInstalledTypescriptUnder stands in for `mise where`: it finds the
// compiler only once the stub mise's install has populated installDir.
func locateInstalledTypescriptUnder(installDir string) func(context.Context, string) (string, bool) {
	return func(context.Context, string) (string, bool) {
		pkgDir := filepath.Join(installDir, "node_modules", "typescript")
		if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
			return "", false
		}
		return pkgDir, true
	}
}

// notYetInstalledDir returns a path the stub mise's install will create, so
// the compiler is absent until that install runs.
func notYetInstalledDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove placeholder install dir: %v", err)
	}
	return dir
}

func writeProjectMiseTypescriptPin(t *testing.T, repo, version string) {
	t.Helper()
	pin := "[tools]\n\"npm:typescript\" = \"" + version + "\"\n"
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte(pin), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}
}
