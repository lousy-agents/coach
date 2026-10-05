package configauthoring

import (
	"strings"
	"testing"
)

func uncoveredLineListsAppsWebLibsxLegacyOnly(t *testing.T, out string) {
	uncoveredLine := lineContaining(out, "no declared layer matches")
	if uncoveredLine == "" {
		t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
	}
	if !strings.Contains(uncoveredLine, "apps/web") {
		t.Fatalf("expected apps/web (matched by no layer) to be listed as uncovered, got:\n%s", uncoveredLine)
	}
	if !strings.Contains(uncoveredLine, "libsx/legacy") {
		t.Fatalf("expected libsx/legacy (a string-prefix sibling of libsLayer's prefix %q, not a real path-segment match) to be listed as uncovered, got:\n%s", "libs", uncoveredLine)
	}
	if strings.Contains(uncoveredLine, "apps/api") || strings.Contains(uncoveredLine, "libs/shared") {
		t.Fatalf("expected only apps/web and libsx/legacy to be listed as uncovered, got:\n%s", uncoveredLine)
	}
}

func printsCandidateSummaryThenCoveragePreviewThenApproval(t *testing.T, out string) {
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
