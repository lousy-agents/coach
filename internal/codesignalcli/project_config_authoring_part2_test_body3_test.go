package codesignalcli

import (
	"strings"
	"testing"
)

func body_projectConfigAuthoringPart2Test_printsEveryCollectedFieldInTheCandidateSummary_133(t *testing.T, out string) {
	rootsLine := lineContaining(out, "roots:")
	if rootsLine == "" {
		t.Fatalf("expected the candidate summary's roots line, got:\n%s", out)
	}
	if !strings.Contains(rootsLine, "apps/api") || !strings.Contains(rootsLine, "apps/web") {
		t.Fatalf("expected the candidate summary's roots line to list the selected roots, got:\n%s", rootsLine)
	}

	if lineContaining(out, "forbidden_imports:") == "" {
		t.Fatalf("expected the candidate summary's forbidden_imports header line, got:\n%s", out)
	}
	if lineContaining(out, "apiLayer -> libsLayer") == "" {
		t.Fatalf("expected the candidate summary to print the declared forbidden pair apiLayer -> libsLayer, got:\n%s", out)
	}
	if lineContaining(out, "libsLayer -> apiLayer") == "" {
		t.Fatalf("expected the candidate summary to print the declared forbidden pair libsLayer -> apiLayer, got:\n%s", out)
	}
	if lineContaining(out, "- apiLayer: apps/api") == "" {
		t.Fatalf("expected the candidate summary's layers block to list apiLayer with its prefix, got:\n%s", out)
	}
	if lineContaining(out, "- libsLayer: libs") == "" {
		t.Fatalf("expected the candidate summary's layers block to list libsLayer with its prefix, got:\n%s", out)
	}

	requiredLayerLine := lineContaining(out, "required_layer:")
	if requiredLayerLine == "" {
		t.Fatalf("expected the candidate summary's required_layer line, got:\n%s", out)
	}
	if !strings.Contains(requiredLayerLine, "apiLayer") {
		t.Fatalf("expected the candidate summary's required_layer line to name apiLayer, got:\n%s", requiredLayerLine)
	}
}

func body_projectConfigAuthoringPart2Test_printsCandidateSummaryThenCoveragePreviewThenApp_167(t *testing.T, out string) {
	candidateIdx := strings.Index(out, "Candidate project config:")
	coverageIdx := strings.Index(out, "Coverage preview:")
	approvalIdx := strings.Index(out, "Type 'approve'")
	if candidateIdx == -1 || coverageIdx == -1 || approvalIdx == -1 {
		t.Fatalf("expected all three markers (candidate summary, coverage preview, approval prompt) to appear, got indices %d/%d/%d, output:\n%s", candidateIdx, coverageIdx, approvalIdx, out)
	}
	if !(candidateIdx < coverageIdx && coverageIdx < approvalIdx) {
		t.Fatalf("expected candidate summary, then coverage preview, then approval prompt in that order (got indices %d, %d, %d), output:\n%s", candidateIdx, coverageIdx, approvalIdx, out)
	}
}
