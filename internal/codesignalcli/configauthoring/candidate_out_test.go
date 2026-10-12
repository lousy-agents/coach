package configauthoring

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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
