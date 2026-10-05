package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func completeCandidateGateOrder(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,2",
		"apiLayer", "apps/api",
		"libsLayer", "libs",
		"",
		"apiLayer", "libsLayer",
		"libsLayer", "apiLayer",
		"",
		"apiLayer",
		"approve",
	)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out)
	}

	t.Run("prints every collected field in the candidate summary", func(t *testing.T) {
		printsEveryCollectedFieldCandidateSummary(t, out)
	})

	t.Run("prints candidate summary, then coverage preview, then approval prompt", func(t *testing.T) {
		printsCandidateSummaryThenCoveragePreviewThenApproval(t, out)
	})
}

func printsEveryCollectedFieldCandidateSummary(t *testing.T, out string) {
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
