package codesignalcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestExecuteSetupRefusesRowWhoseExecutableDiffersFromItsMapKey pins a
// defense-in-depth guard that has no reachable counterexample in today's
// frozen matrix (SA-280-012): all three real rows key setupCommandTemplates
// by the same string as their own executable field. It exists for the day a
// future row (for example packageManagerKindYarn, already declared but not
// yet wired into the matrix) does not share that property. setupCommandTemplates
// is mutated directly and restored via t.Cleanup, since the public API
// (BuildSetupPreview) can never itself produce such a mismatched row.
func TestExecuteSetupRefusesRowWhoseExecutableDiffersFromItsMapKey(t *testing.T) {
	const fakeKind = "coach-test-fake-kind"
	fakeArgs := []string{"--frozen"}
	original, hadOriginal := setupCommandTemplates[fakeKind]
	setupCommandTemplates[fakeKind] = setupCommandTemplate{
		executable: "coach-test-fake-real-binary", // deliberately not fakeKind
		args:       fakeArgs,
	}
	t.Cleanup(func() {
		if hadOriginal {
			setupCommandTemplates[fakeKind] = original
		} else {
			delete(setupCommandTemplates, fakeKind)
		}
	})

	// A stub literally named fakeKind: if ExecuteSetup ever spawns
	// preview.Executable without checking it against the row's own
	// executable field, this is what would run.
	stubDir := t.TempDir()
	marker := filepath.Join(stubDir, "invoked.marker")
	script := "#!/bin/sh\n: > " + marker + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(stubDir, fakeKind), []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	workDir := t.TempDir()
	preview := SetupPreview{
		Executable:       fakeKind,
		Args:             fakeArgs,
		WorkingDirectory: workDir,
		Timeout:          SetupPreviewTimeout,
	}

	result, err := ExecuteSetup(context.Background(), preview, true)
	if !errors.Is(err, ErrSetupExecutionUnverifiedCommand) {
		t.Fatalf("err = %v, want ErrSetupExecutionUnverifiedCommand", err)
	}
	if result.Executable != "" || result.Args != nil || result.WorkingDirectory != "" || result.Succeeded || result.TimedOut {
		t.Fatalf("result = %+v, want zero value", result)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("a row whose executable differs from its own map key must never reach exec.CommandContext, but %s was invoked", fakeKind)
	}
}
