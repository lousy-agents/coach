package codesignalcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readinessWithPrepareCompilerAction(action ReadinessNextAction) *ReadinessResult {
	return &ReadinessResult{NextActions: []ReadinessNextAction{action}}
}

func TestRunPrepareCompilerMiseSetupReportsNoChoicesOffered(t *testing.T) {
	cases := map[string]*ReadinessResult{
		"nil readiness":                nil,
		"no next actions at all":       {},
		"prepare_compiler not present": readinessWithPrepareCompilerAction(ReadinessNextAction{Kind: "install_supported_runtime", Executable: false}),
		"prepare_compiler not executable": readinessWithPrepareCompilerAction(ReadinessNextAction{
			Kind: nextActionKindPrepareCompiler, Executable: false, Choices: []string{compilerOriginMiseProject},
		}),
		"choices restricted to a non-mise kind only": readinessWithPrepareCompilerAction(ReadinessNextAction{
			Kind: nextActionKindPrepareCompiler, Executable: true, Choices: []string{"npm_project"},
		}),
	}

	for name, readiness := range cases {
		t.Run(name, func(t *testing.T) {
			result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", "", readiness, strings.NewReader(""), &bytes.Buffer{})
			if !result.NoChoicesOffered {
				t.Fatalf("NoChoicesOffered = false, want true: %+v", result)
			}
			if result.Cancelled || result.Attempted || result.Trusted || result.Succeeded {
				t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
			}
		})
	}
}

func TestRunPrepareCompilerMiseSetupCancelsOnUnrecognizedSelection(t *testing.T) {
	readiness := readinessWithPrepareCompilerAction(ReadinessNextAction{
		Kind: nextActionKindPrepareCompiler, Executable: true,
		Choices: []string{compilerOriginMiseProject, compilerOriginMiseGlobal},
	})
	var transcript bytes.Buffer

	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", "", readiness, strings.NewReader("not-a-choice\n"), &transcript)

	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Attempted {
		t.Fatalf("Attempted = true, want false: no install must ever run after a cancelled selection: %+v", result)
	}
	if strings.Contains(transcript.String(), "Executable: mise") {
		t.Fatalf("transcript reached the install preview despite an unrecognized selection (no default may be assumed): %s", transcript.String())
	}
}

func TestRunPrepareCompilerMiseSetupCancelsOnDeclinedConfirmation(t *testing.T) {
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

func shellQuoteForTest(path string) string {
	return "\"" + path + "\""
}

func writeFakeInstalledTypescriptForTest(t *testing.T, installDir, version string) {
	t.Helper()
	pkgDir := filepath.Join(installDir, "node_modules", "typescript")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"typescript","version":"`+version+`"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeTypescriptUnscopedName())
	if err := os.MkdirAll(nativeDir, 0o755); err != nil {
		t.Fatalf("mkdir native: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(`{"name":"`+NativeTypescriptPackageName()+`","version":"`+version+`"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write native package.json: %v", err)
	}
}

// TestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun drives
// RunPrepareCompilerMiseSetup's full confirmed-install path (AC-SET-3,
// AC-SET-6): locateMiseTypescriptInstall only finds the compiler once
// installDir exists, which only happens once the stub mise's own `install`
// invocation has actually run -- so the pre/post-install state genuinely
// differs the way a real mise would, without any network dependency. It
// also directly checks evaluateCompilerOrigins' own winner.origin after the
// rerun, since the frozen checks.compiler JSON contract does not itself
// expose an origin field for a passing compiler check (AC-SET-23's
// underlying fact, verified at the layer that actually carries it).
func TestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun(t *testing.T) {
	installDir := t.TempDir()
	if err := os.RemoveAll(installDir); err != nil {
		t.Fatalf("remove placeholder install dir: %v", err)
	}
	writeStatefulStubMiseOnPath(t, installDir, "7.0.2")

	originalLocate := locateMiseTypescriptInstall
	defer func() { locateMiseTypescriptInstall = originalLocate }()
	locateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
		pkgDir := filepath.Join(installDir, "node_modules", "typescript")
		if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
			return "", false
		}
		return pkgDir, true
	}

	repo := newTempGitRepoT(t)
	revision := commitFileT(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

	before, err := CheckProjectReadiness(repo, revision, "")
	if err != nil {
		t.Fatalf("CheckProjectReadiness (before): %v", err)
	}
	if before.Checks.Compiler.State != ReadinessFail || before.Checks.Compiler.Code != GapTypescriptCompilerMissing {
		t.Fatalf("sanity: expected an initial typescript_compiler_missing gap, got %+v", before.Checks.Compiler)
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
	if !result.Succeeded {
		t.Fatalf("Succeeded = false, want true: %+v", result)
	}
	if result.Origin != compilerOriginMiseProject {
		t.Fatalf("Origin = %q, want %q", result.Origin, compilerOriginMiseProject)
	}
	if result.PostInstallReadiness == nil {
		t.Fatalf("PostInstallReadiness is nil, want the rerun result (AC-SET-6)")
	}
	if result.PostInstallReadiness.Checks.Compiler.State != ReadinessPass {
		t.Fatalf("rerun compiler state = %q, want pass: %+v", result.PostInstallReadiness.Checks.Compiler.State, result.PostInstallReadiness.Checks.Compiler)
	}
	if result.PostInstallReadiness.Checks.Compiler.Version != "7.0.2" {
		t.Fatalf("rerun compiler version = %q, want 7.0.2", result.PostInstallReadiness.Checks.Compiler.Version)
	}

	aggregate := evaluateCompilerOrigins(repo, nil)
	if aggregate.winner == nil {
		t.Fatalf("evaluateCompilerOrigins winner is nil after a successful install")
	}
	if aggregate.winner.origin != compilerOriginMiseProject {
		t.Fatalf("winner.origin = %q, want %q (AC-SET-23's underlying fact)", aggregate.winner.origin, compilerOriginMiseProject)
	}
}

// writeFailingInstallStubMiseOnPath puts a `mise` executable on t's PATH
// whose `install` subcommand always exits 1 without moving anything into
// place, modeling a genuine `mise install` failure (AC-SET-7) rather than a
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

// TestRunPrepareCompilerMiseSetupReportsRequestedVersionOnInstallFailure
// proves the AC-SET-7 "may have changed" precondition always names a real
// tool spec: when `mise install` itself fails, installMiseTypescript never
// observes a candidate version (miseInstallResult.Version stays ""), so the
// result must fall back to the version that was actually requested rather
// than surfacing an empty string to a caller building a failure message.
func TestRunPrepareCompilerMiseSetupReportsRequestedVersionOnInstallFailure(t *testing.T) {
	writeFailingInstallStubMiseOnPath(t)

	repo := newTempGitRepoT(t)
	revision := commitFileT(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

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

// TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil
// proves the other half of AC-22's consumption contract: when
// action.Choices is nil (no package-manager-adapter restriction has run),
// miseChoicesForPrepareCompiler recomputes the same evaluateMiseSetupChoices
// verification CheckProjectReadiness itself uses, rather than assuming
// nothing is offered.
func TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil(t *testing.T) {
	originalProbeVersion := probeMiseToolVersion
	originalGlobalHazard := probeMiseGlobalConfigHazard
	defer func() {
		probeMiseToolVersion = originalProbeVersion
		probeMiseGlobalConfigHazard = originalGlobalHazard
	}()
	probeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
	probeMiseGlobalConfigHazard = func(context.Context) bool { return false }

	repo := newTempGitRepoT(t)
	revision := commitFileT(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	revision = commitFileT(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	action := ReadinessNextAction{Kind: nextActionKindPrepareCompiler, Executable: true}
	got := miseChoicesForPrepareCompiler(repo, revision, "", action)

	want := []string{compilerOriginMiseProject, compilerOriginMiseGlobal}
	if len(got) != len(want) {
		t.Fatalf("miseChoicesForPrepareCompiler = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("miseChoicesForPrepareCompiler = %#v, want %#v", got, want)
		}
	}
}

func TestFilterMiseChoiceKindsDropsNonMiseChoices(t *testing.T) {
	got := filterMiseChoiceKinds([]string{"npm_project", compilerOriginMiseProject, "yarn", compilerOriginMiseGlobal})
	want := []string{compilerOriginMiseProject, compilerOriginMiseGlobal}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("filterMiseChoiceKinds = %#v, want %#v", got, want)
	}
}
