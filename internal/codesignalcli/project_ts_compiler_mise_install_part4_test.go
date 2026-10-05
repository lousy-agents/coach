package codesignalcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestRunPrepareCompilerMiseSetupCancelsOnUnrecognizedSelection(t *testing.T) {
	readiness := readinessWithPrepareCompilerAction(projectreadiness.NextAction{
		Kind: projectreadiness.NextActionPrepareCompiler, Executable: true,
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
	got := filterMiseChoiceKinds([]string{"npm_project", compilerOriginMiseProject, "yarn", compilerOriginMiseGlobal})
	want := []string{compilerOriginMiseProject, compilerOriginMiseGlobal}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("filterMiseChoiceKinds = %#v, want %#v", got, want)
	}
}
