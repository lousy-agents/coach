package configauthoring

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovedAndOutputSet_WritesCreateOnly(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("target does not yet exist: it is created with the exact candidate content", func(t *testing.T) {
		targetDoesNotYetExistCreatedExactCandidate(t, discovered)
	})

	t.Run("target already exists: the write is refused and the existing content is left untouched", func(t *testing.T) {
		targetAlreadyExistsWriteRefusedExistingContentLeft(t, discovered)
	})

	t.Run("output path escapes the repository root: the write is refused and nothing is created outside dir", func(t *testing.T) {
		outputPathEscapesRepositoryRootWriteRefusedNothing(t, discovered)
	})

	t.Run("output path contains a .git component: the write is refused and nothing is created inside .git", func(t *testing.T) {
		outputPathContainsGitComponentWriteRefusedNothing(t, discovered)
	})

	t.Run("output path's parent directory does not exist: the write is refused and nothing is created", func(t *testing.T) {
		outputPathsParentDirectoryDoesNotExistWrite(t, discovered)
	})

	t.Run("output path's parent is a symlink: the write is refused and nothing is created through it", func(t *testing.T) {
		outputPathsParentSymlinkWriteRefusedNothingCreated(t, discovered)
	})
}

func targetDoesNotYetExistCreatedExactCandidate(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

	result := Author(dir, in, out, out, discovered, outputPath, true)

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

	want := approvedCandidateBytes(t, projectconfig.Config{
		Roots:  []string{"apps/api"},
		Layers: []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}},
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
