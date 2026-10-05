package codesignalcli

import (
	"bytes"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart3Test_targetDoesNotYetExistItIsCreatedWithTheExactCand_17(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outputPath := "config/project.json"
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("failed to prepare parent directory: %v", err)
	}
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain",
		"internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := AuthorProjectConfig(dir, in, out, out, discovered, outputPath, true)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
	}
	if result.ValidationError != nil {
		t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
	}
	if result.WriteError != nil {
		t.Fatalf("expected WriteError = nil, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a target that did not previously exist")
	}

	want := approvedCandidateBytes(t, projectConfig{
		Roots:  []string{"apps/api"},
		Layers: []projectConfigLayer{{Name: "domain", Prefixes: []string{"internal/domain"}}},
	})
	if !bytes.Equal(result.Document, want) {
		t.Fatalf("Document = %s, want %s", result.Document, want)
	}

	got, err := os.ReadFile(filepath.Join(dir, outputPath))
	if err != nil {
		t.Fatalf("failed to read back written file: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("written file content = %s, want byte-for-byte %s", got, want)
	}
}
