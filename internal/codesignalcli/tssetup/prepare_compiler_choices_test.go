package tssetup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
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

// TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil
// proves that when action.Choices is nil (no package-manager-adapter
// restriction has run), miseChoicesForPrepareCompiler recomputes the same
// tstoolchain.EvaluateMiseSetupChoices verification projectcheck.Run itself uses,
// rather than assuming nothing is offered.
func TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil(t *testing.T) {
	originalProbeVersion := tstoolchain.ProbeMiseToolVersion
	originalGlobalHazard := tstoolchain.ProbeMiseGlobalConfigHazard
	originalDetect := tstoolchain.DetectGlobalMiseTypescriptVersion
	defer func() {
		tstoolchain.ProbeMiseToolVersion = originalProbeVersion
		tstoolchain.ProbeMiseGlobalConfigHazard = originalGlobalHazard
		tstoolchain.DetectGlobalMiseTypescriptVersion = originalDetect
	}()
	tstoolchain.ProbeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
	tstoolchain.ProbeMiseGlobalConfigHazard = func(context.Context) bool { return false }
	tstoolchain.DetectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return tstoolchain.NewestSupportedTypescriptVersion(), true
	}

	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	revision = gitfixture.CommitFile(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

	action := projectreadiness.NextAction{Kind: projectreadiness.NextActionPrepareCompiler, Executable: true}
	got := miseChoicesForPrepareCompiler(repo, revision, "", action)

	want := []string{tstoolchain.OriginMiseProject, tstoolchain.OriginMiseGlobal}
	if len(got) != len(want) {
		t.Fatalf("miseChoicesForPrepareCompiler = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("miseChoicesForPrepareCompiler = %#v, want %#v", got, want)
		}
	}
}

func TestRunPrepareCompilerMiseSetupReportsNoChoicesOffered(t *testing.T) {
	cases := map[string]*projectreadiness.Result{
		"nil readiness":                nil,
		"no next actions at all":       {Checks: projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Pass}}},
		"prepare_compiler not present": readinessWithPrepareCompilerAction(projectreadiness.NextAction{Kind: "install_supported_runtime", Executable: false}),
		"prepare_compiler not executable": readinessWithPrepareCompilerAction(projectreadiness.NextAction{
			Kind: projectreadiness.NextActionPrepareCompiler, Executable: false, Choices: []string{tstoolchain.OriginMiseProject},
		}),
		"choices restricted to a non-mise kind only": readinessWithPrepareCompilerAction(projectreadiness.NextAction{
			Kind: projectreadiness.NextActionPrepareCompiler, Executable: true, Choices: []string{"npm_project"},
		}),
	}

	for name, readiness := range cases {
		t.Run(name, func(t *testing.T) {
			checkRunPrepareCompilerMiseSetupReports(t, readiness)
		})
	}
}

func checkRunPrepareCompilerMiseSetupReports(t *testing.T, readiness *projectreadiness.Result) {
	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", "", readiness, strings.NewReader(""), &bytes.Buffer{})
	if !result.NoChoicesOffered {
		t.Fatalf("NoChoicesOffered = false, want true: %+v", result)
	}
	if result.Cancelled || result.Attempted || result.Trusted || result.Succeeded {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
}

func TestRunPrepareCompilerMiseSetupCancelsOnUnrecognizedSelection(t *testing.T) {
	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true,
		Choices: []string{tstoolchain.OriginMiseProject, tstoolchain.OriginMiseGlobal},
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

func TestFilterMiseChoiceKindsDropsNonMiseChoices(t *testing.T) {
	got := filterMiseChoiceKinds([]string{"npm_project", tstoolchain.OriginMiseProject, "yarn", tstoolchain.OriginMiseGlobal})
	want := []string{tstoolchain.OriginMiseProject, tstoolchain.OriginMiseGlobal}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("filterMiseChoiceKinds = %#v, want %#v", got, want)
	}
}
