package configauthoring

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func targetAlreadyExistsWriteRefusedExistingContentLeft(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outputPath := "project.json"
	preexisting := []byte("this is not a project config and must not be overwritten\n")
	if err := os.WriteFile(filepath.Join(dir, outputPath), preexisting, 0o644); err != nil {
		t.Fatalf("failed to seed a pre-existing output file: %v", err)
	}

	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := Author(dir, in, out, out, discovered, outputPath, true)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
	}
	if !result.OutputExists {
		t.Fatalf("expected OutputExists = true for a target that already existed")
	}
	if result.WriteError != nil {
		t.Fatalf("expected WriteError = nil when the refusal is reported via OutputExists, got %v", result.WriteError)
	}

	got, err := os.ReadFile(filepath.Join(dir, outputPath))
	if err != nil {
		t.Fatalf("failed to read back the pre-existing file: %v", err)
	}
	if !bytes.Equal(got, preexisting) {
		t.Fatalf("expected the pre-existing file to be left untouched, got %s, want %s", got, preexisting)
	}
}

func outputPathsParentDirectoryDoesNotExistWrite(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outputPath := "missing-parent/project.json"
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := Author(dir, in, out, out, discovered, outputPath, true)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
	}

	if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `parent directory "missing-parent" does not exist`) {
		t.Fatalf("expected WriteError to report the missing-parent-directory rejection, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected missing-parent path")
	}

	if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q, stat err = %v", outputPath, err)
	}
}
