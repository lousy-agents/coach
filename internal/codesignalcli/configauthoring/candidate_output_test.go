package configauthoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovedAndOutputUnset_WriteFailureIsReportedNotSilentlySuccessful(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}
	writeErr := errors.New("write: broken pipe")
	transcript := &bytes.Buffer{}
	candidateOut := &alwaysErrorWriter{err: writeErr}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := Author(t.TempDir(), in, transcript, candidateOut, discovered, "", false)

	if !result.Approved {
		t.Fatalf("expected Approved = true (the candidate itself is valid), got false; transcript:\n%s", transcript.String())
	}
	if result.ValidationError != nil {
		t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
	}
	if result.WriteError == nil {
		t.Fatalf("expected WriteError to report the failed write to candidateOut, got nil")
	}
	if !errors.Is(result.WriteError, writeErr) {
		t.Fatalf("WriteError = %v, want it to wrap %v", result.WriteError, writeErr)
	}
}

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

func outputPathEscapesRepositoryRootWriteRefusedNothing(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outputPath := "../escaped.json"
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
	if result.WriteError == nil {
		t.Fatalf("expected WriteError for an output path that escapes the repository root, got nil")
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected escaping path")
	}

	escapedTarget := filepath.Join(filepath.Dir(dir), "escaped.json")
	if _, err := os.Stat(escapedTarget); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q outside the repository root, stat err = %v", escapedTarget, err)
	}
}

func outputPathContainsGitComponentWriteRefusedNothing(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("failed to prepare a real .git directory: %v", err)
	}
	outputPath := ".git/config.json"
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
	if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `must not contain a ".git" path component`) {
		t.Fatalf("expected WriteError to report the .git-component rejection, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected .git-component path")
	}

	if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q, stat err = %v", outputPath, err)
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

func outputPathsParentSymlinkWriteRefusedNothingCreated(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatalf("failed to prepare a symlinked parent: %v", err)
	}
	outputPath := "link/project.json"
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

	if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `parent path component "link" is a symlink`) {
		t.Fatalf("expected WriteError to report the symlinked-parent rejection, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected symlinked-parent path")
	}

	if _, err := os.Stat(filepath.Join(outside, "project.json")); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q outside the repository root, stat err = %v", filepath.Join(outside, "project.json"), err)
	}
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

func TestAuthorProjectConfig_ApprovedAndOutputUnset_WritesCandidateToCandidateOutOnly(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}
	transcript := &bytes.Buffer{}
	candidateOut := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := Author(t.TempDir(), in, transcript, candidateOut, discovered, "", false)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; transcript:\n%s", transcript.String())
	}
	if result.ValidationError != nil {
		t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
	}

	want := approvedCandidateBytes(t, projectconfig.Config{
		Roots:  []string{"apps/api"},
		Layers: []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}},
	})
	if !bytes.Equal(result.Document, want) {
		t.Fatalf("Document = %s, want %s", result.Document, want)
	}

	t.Run("candidateOut is exactly the schema-1 document", func(t *testing.T) {
		candidateOutExactlySchema1Document(t, candidateOut, want)
	})
	t.Run("transcript writer is non-empty", func(t *testing.T) {
		transcriptWriterNonEmpty(t, transcript)
	})
	t.Run("transcript writer does not contain schema_version", func(t *testing.T) {
		transcriptWriterDoesNotContainSchemaVersion(t, transcript)
	})

	var decoded projectconfig.Config
	if err := json.Unmarshal(candidateOut.Bytes(), &decoded); err != nil {
		t.Fatalf("expected the emitted document to be valid JSON, got error %v decoding %q", err, candidateOut.String())
	}
	if decoded.SchemaVersion != "1" {
		t.Fatalf("schema_version = %q, want %q", decoded.SchemaVersion, "1")
	}
	if !equalStringSlices(decoded.Roots, []string{"apps/api"}) {
		t.Fatalf("decoded roots = %v, want [apps/api]", decoded.Roots)
	}
	if !equalLayers(decoded.Layers, []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}) {
		t.Fatalf("decoded layers = %+v, want the declared domain layer", decoded.Layers)
	}
}

func candidateOutExactlySchema1Document(t *testing.T, candidateOut *bytes.Buffer, want []byte) {
	if !bytes.Equal(candidateOut.Bytes(), want) {
		t.Fatalf("candidateOut = %q, want exactly the candidate document %q", candidateOut.String(), want)
	}
}

func transcriptWriterNonEmpty(t *testing.T, transcript *bytes.Buffer) {
	if transcript.Len() == 0 {
		t.Fatalf("expected the interactive transcript to be written to the transcript writer, got none")
	}
}

func transcriptWriterDoesNotContainSchemaVersion(t *testing.T, transcript *bytes.Buffer) {
	if strings.Contains(transcript.String(), `"schema_version"`) {
		t.Fatalf("expected the candidate document to never appear in the transcript writer, got:\n%s", transcript.String())
	}
}
