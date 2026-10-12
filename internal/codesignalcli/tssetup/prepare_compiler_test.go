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
