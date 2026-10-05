package codesignalcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
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
	nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeTypescriptUnscopedName())
	if err := os.MkdirAll(nativeDir, 0o755); err != nil {
		t.Fatalf("mkdir native: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(`{"name":"`+NativeTypescriptPackageName()+`","version":"`+version+`"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write native package.json: %v", err)
	}
}

// TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil
// proves that when action.Choices is nil (no package-manager-adapter
// restriction has run), miseChoicesForPrepareCompiler recomputes the same
// evaluateMiseSetupChoices verification CheckProjectReadiness itself uses,
// rather than assuming nothing is offered.
func TestMiseChoicesForPrepareCompilerFallsBackToVerifiedChoicesWhenActionChoicesNil(t *testing.T) {
	originalProbeVersion := probeMiseToolVersion
	originalGlobalHazard := probeMiseGlobalConfigHazard
	originalDetect := detectGlobalMiseTypescriptVersion
	defer func() {
		probeMiseToolVersion = originalProbeVersion
		probeMiseGlobalConfigHazard = originalGlobalHazard
		detectGlobalMiseTypescriptVersion = originalDetect
	}()
	probeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
	probeMiseGlobalConfigHazard = func(context.Context) bool { return false }
	detectGlobalMiseTypescriptVersion = func(context.Context) (string, bool) {
		return newestSupportedTypescriptVersion(), true
	}

	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	revision = gitfixture.CommitFile(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
	if err := os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		t.Fatalf("write mise.toml: %v", err)
	}

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

func TestRunPrepareCompilerMiseSetupReportsNoChoicesOffered(t *testing.T) {
	cases := map[string]*ReadinessResult{
		"nil readiness":                nil,
		"no next actions at all":       {Checks: ReadinessChecks{Policy: ReadinessCheck{State: ReadinessPass}}},
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
			body_projectTsCompilerMiseInstallPart3Test_85(t, readiness)
		})
	}
}
