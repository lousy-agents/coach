package codesignalcli

import (
	"context"
	"os"
	"path/filepath"
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
			body_projectTsCompilerMiseInstallPart3Test_85(t, readiness)
		})
	}
}
