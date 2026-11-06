package tssetup

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

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
	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", policyref.Ref{}, readiness, strings.NewReader(""), &bytes.Buffer{})
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

	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", policyref.Ref{}, readiness, strings.NewReader("not-a-choice\n"), &transcript)

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
