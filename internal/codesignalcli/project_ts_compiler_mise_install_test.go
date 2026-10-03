package codesignalcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPrepareCompilerMiseSetupCancelsOnDeclinedConfirmation(t *testing.T) {
	originalDetect := detectGlobalMiseTypescriptVersion
	defer func() { detectGlobalMiseTypescriptVersion = originalDetect }()
	detectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return newestSupportedTypescriptVersion(), true
	}

	readiness := readinessWithPrepareCompilerAction(ReadinessNextAction{
		Kind: nextActionKindPrepareCompiler, Executable: true,
		Choices: []string{compilerOriginMiseProject, compilerOriginMiseGlobal},
	})
	var transcript bytes.Buffer

	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", "", readiness, strings.NewReader("mise_global\nno\n"), &transcript)

	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Choice != compilerOriginMiseGlobal {
		t.Fatalf("Choice = %q, want %q", result.Choice, compilerOriginMiseGlobal)
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

	repo := newTempGitRepoT(t)
	revision := commitFileT(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

	readiness := readinessWithPrepareCompilerAction(ReadinessNextAction{
		Kind: nextActionKindPrepareCompiler, Executable: true, Choices: []string{compilerOriginMiseProject},
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
	if result.Version != newestSupportedTypescriptVersion() {
		t.Fatalf("Version = %q, want the requested version %q, not empty: %+v", result.Version, newestSupportedTypescriptVersion(), result)
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
// (Checks.Policy.State: ReadinessPass): every case here is exercising
// prepare_compiler's own selection/install machinery, not the policy gate
// (see TestRunPrepareCompilerMiseSetupRequiresPolicyFirst for that), so a
// zero-value Policy state (which RunPrepareCompilerMiseSetup treats as
// not-yet-passed, fail-closed) would wrongly report PolicyRequired here
// instead.
func readinessWithPrepareCompilerAction(action ReadinessNextAction) *ReadinessResult {
	return &ReadinessResult{
		Checks:      ReadinessChecks{Policy: ReadinessCheck{State: ReadinessPass}},
		NextActions: []ReadinessNextAction{action},
	}
}

func shellQuoteForTest(path string) string {
	return "\"" + path + "\""
}
