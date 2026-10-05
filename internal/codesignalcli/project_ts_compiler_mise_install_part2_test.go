package codesignalcli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

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
	(&sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun0{installDir: installDir, t: t}).call()

	writeStatefulStubMiseOnPath(t, installDir, "7.0.2")

	originalLocate := tstoolchain.LocateMiseTypescriptInstall
	defer func() { tstoolchain.LocateMiseTypescriptInstall = originalLocate }()
	tstoolchain.LocateMiseTypescriptInstall = (&sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun{installDir: installDir}).call

	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	(&sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun1{repo: repo, t: t}).call()

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
