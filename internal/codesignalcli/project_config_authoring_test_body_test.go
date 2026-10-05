package codesignalcli

import (
	"bytes"
	"strings"
	"testing"
)

func body_projectConfigAuthoringTest_candidateOutIsExactlyTheSchema1Document_88(t *testing.T, candidateOut *bytes.Buffer, want []byte) {
	if !bytes.Equal(candidateOut.Bytes(), want) {
		t.Fatalf("candidateOut = %q, want exactly the candidate document %q", candidateOut.String(), want)
	}
}

func body_projectConfigAuthoringTest_transcriptWriterIsNonEmpty_93(t *testing.T, transcript *bytes.Buffer) {
	if transcript.Len() == 0 {
		t.Fatalf("expected the interactive transcript to be written to the transcript writer, got none")
	}
}

func body_projectConfigAuthoringTest_transcriptWriterDoesNotContainSchemaVersion_98(t *testing.T, transcript *bytes.Buffer) {
	if strings.Contains(transcript.String(), `"schema_version"`) {
		t.Fatalf("expected the candidate document to never appear in the transcript writer, got:\n%s", transcript.String())
	}
}
