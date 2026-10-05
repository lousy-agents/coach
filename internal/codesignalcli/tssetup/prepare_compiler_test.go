package tssetup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

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

// TestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun drives
// RunPrepareCompilerMiseSetup's full confirmed-install path:
// tstoolchain.LocateMiseTypescriptInstall only finds the compiler once installDir
// exists, which only happens once the stub mise's own `install` invocation
// has actually run -- so the pre/post-install state genuinely differs the
// way a real mise would, without any network dependency. It also directly
// checks EvaluateOrigins' own winner.origin after the rerun, since
// the frozen checks.compiler JSON contract does not itself expose an origin
// field for a passing compiler check.
func TestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun(t *testing.T) {
	installDir := t.TempDir()
	if err := os.RemoveAll(installDir); err != nil {
		t.Fatalf("remove placeholder install dir: %v", err)
	}

	writeStatefulStubMiseOnPath(t, installDir, "7.0.2")

	originalLocate := tstoolchain.LocateMiseTypescriptInstall
	defer func() { tstoolchain.LocateMiseTypescriptInstall = originalLocate }()
	tstoolchain.LocateMiseTypescriptInstall = locateInstalledTypescriptUnder(installDir)

	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

	before, err := projectcheck.Run(repo, revision, "")
	if err != nil {
		t.Fatalf("CheckProjectReadiness (before): %v", err)
	}
	if before.Checks.Compiler.State != projectreadiness.Fail || before.Checks.Compiler.Code != projectreadiness.GapTypescriptCompilerMissing {
		t.Fatalf("sanity: expected an initial typescript_compiler_missing gap, got %+v", before.Checks.Compiler)
	}

	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true, Choices: []string{tstoolchain.OriginMiseProject},
	})
	var transcript bytes.Buffer

	result := RunPrepareCompilerMiseSetup(context.Background(), repo, revision, "", readiness, strings.NewReader("mise_project\ninstall\n"), &transcript)

	if !result.Trusted {
		t.Fatalf("Trusted = false, want true: %+v transcript=%s", result, transcript.String())
	}
	if !result.Attempted {
		t.Fatalf("Attempted = false, want true: %+v", result)
	}
	if !result.Succeeded {
		t.Fatalf("Succeeded = false, want true: %+v", result)
	}
	if result.Origin != tstoolchain.OriginMiseProject {
		t.Fatalf("Origin = %q, want %q", result.Origin, tstoolchain.OriginMiseProject)
	}
	if result.PostInstallReadiness == nil {
		t.Fatalf("PostInstallReadiness is nil, want the rerun result")
	}
	if result.PostInstallReadiness.Checks.Compiler.State != projectreadiness.Pass {
		t.Fatalf("rerun compiler state = %q, want pass: %+v", result.PostInstallReadiness.Checks.Compiler.State, result.PostInstallReadiness.Checks.Compiler)
	}
	if result.PostInstallReadiness.Checks.Compiler.Version != "7.0.2" {
		t.Fatalf("rerun compiler version = %q, want 7.0.2", result.PostInstallReadiness.Checks.Compiler.Version)
	}

	aggregate := tstoolchain.EvaluateOrigins(repo, nil)
	if aggregate.Winner == nil {
		t.Fatalf("evaluateCompilerOrigins winner is nil after a successful install")
	}
	if aggregate.Winner.Origin != tstoolchain.OriginMiseProject {
		t.Fatalf("winner.origin = %q, want %q", aggregate.Winner.Origin, tstoolchain.OriginMiseProject)
	}
}

func TestRunPrepareCompilerMiseSetupCancelsOnDeclinedConfirmation(t *testing.T) {
	originalDetect := tstoolchain.DetectGlobalMiseTypescriptVersion
	defer func() { tstoolchain.DetectGlobalMiseTypescriptVersion = originalDetect }()
	tstoolchain.DetectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return tstoolchain.NewestSupportedTypescriptVersion(), true
	}

	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true,
		Choices: []string{tstoolchain.OriginMiseProject, tstoolchain.OriginMiseGlobal},
	})
	var transcript bytes.Buffer

	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", "", readiness, strings.NewReader("mise_global\nno\n"), &transcript)

	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Choice != tstoolchain.OriginMiseGlobal {
		t.Fatalf("Choice = %q, want %q", result.Choice, tstoolchain.OriginMiseGlobal)
	}
	if result.Attempted {
		t.Fatalf("Attempted = true, want false: a declined confirmation must never invoke install: %+v", result)
	}
	preview := transcript.String()
	for _, field := range []string{
		"Executable: mise",
		"Arguments: install npm:typescript@7.0.2",
		"Working directory:",
		"Expected mise changes:",
		"Network use:",
		"Lifecycle-script policy:",
		"Timeout: 5m0s",
	} {
		if !strings.Contains(preview, field) {
			t.Fatalf("preview missing field %q, got:\n%s", field, preview)
		}
	}
}

// TestRunPrepareCompilerMiseSetupReportsRequestedVersionOnInstallFailure
// proves a failure message always names a real tool spec: when `mise
// install` itself fails, installMiseTypescript never observes a candidate
// version (miseInstallResult.Version stays ""), so the result must fall
// back to the version that was actually requested rather than surfacing an
// empty string to a caller building a failure message.
func TestRunPrepareCompilerMiseSetupReportsRequestedVersionOnInstallFailure(t *testing.T) {
	writeFailingInstallStubMiseOnPath(t)

	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true, Choices: []string{tstoolchain.OriginMiseProject},
	})
	var transcript bytes.Buffer

	result := RunPrepareCompilerMiseSetup(context.Background(), repo, revision, "", readiness, strings.NewReader("mise_project\ninstall\n"), &transcript)

	if !result.Trusted {
		t.Fatalf("Trusted = false, want true: %+v transcript=%s", result, transcript.String())
	}
	if !result.Attempted {
		t.Fatalf("Attempted = false, want true: %+v", result)
	}
	if result.Succeeded {
		t.Fatalf("Succeeded = true, want false: the stub install exits 1: %+v", result)
	}
	if result.Version != tstoolchain.NewestSupportedTypescriptVersion() {
		t.Fatalf("Version = %q, want the requested version %q, not empty: %+v", result.Version, tstoolchain.NewestSupportedTypescriptVersion(), result)
	}
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

// readinessWithPrepareCompilerAction models an already-passed policy check
// (Checks.Policy.State: projectreadiness.Pass): every case here is exercising
// prepare_compiler's own selection/install machinery, not the policy gate
// (see TestRunPrepareCompilerMiseSetupRequiresPolicyFirst for that), so a
// zero-value Policy state (which RunPrepareCompilerMiseSetup treats as
// not-yet-passed, fail-closed) would wrongly report PolicyRequired here
// instead.
func readinessWithPrepareCompilerAction(action projectreadiness.NextAction) *projectreadiness.Result {
	return &projectreadiness.Result{
		Checks:      projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Pass}},
		NextActions: []projectreadiness.NextAction{action},
	}
}

func shellQuoteForTest(path string) string {
	return "\"" + path + "\""
}
