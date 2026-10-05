package codesignalcli

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func body_projectTsCompilerMiseVersionTest_22(t *testing.T, tc struct {
	name   string
	input  string
	want   string
	wantOK bool
}) {
	got, ok := miseToolVersionToken(tc.input)
	if ok != tc.wantOK || got != tc.want {
		t.Errorf("miseToolVersionToken(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.want, tc.wantOK)
	}
}

func body_projectTsCompilerMiseVersionTest_inRowVersionIsReady_41(t *testing.T) {
	stubProbeMiseToolVersion(t, "2026.9.5 linux-x64 (2026-09-10)", true)
	got := evaluateMiseToolVersionReadiness(context.Background())
	if !got.ready || got.code != "" {
		t.Errorf("evaluateMiseToolVersionReadiness() = %+v, want ready with no code", got)
	}
}

func body_projectTsCompilerMiseVersionTest_outOfRowVersionIsUnsupported_49(t *testing.T) {
	stubProbeMiseToolVersion(t, "2025.1.0 linux-x64 (2025-01-01)", true)
	got := evaluateMiseToolVersionReadiness(context.Background())
	if got.ready || got.code != projectreadiness.GapPackageManagerVersionUnsupported {
		t.Errorf("evaluateMiseToolVersionReadiness() = %+v, want unsupported", got)
	}
}

func body_projectTsCompilerMiseVersionTest_failedProbeIsUnverifiable_57(t *testing.T) {
	stubProbeMiseToolVersion(t, "", false)
	got := evaluateMiseToolVersionReadiness(context.Background())
	if got.ready || got.code != projectreadiness.GapPackageManagerVersionUnverifiable {
		t.Errorf("evaluateMiseToolVersionReadiness() = %+v, want unverifiable", got)
	}
}

func body_projectTsCompilerMiseVersionTest_emptySuccessfulProbeOutputIsUnverifiableNotUnsup_65(t *testing.T) {
	stubProbeMiseToolVersion(t, "   ", true)
	got := evaluateMiseToolVersionReadiness(context.Background())
	if got.ready || got.code != projectreadiness.GapPackageManagerVersionUnverifiable {
		t.Errorf("evaluateMiseToolVersionReadiness() = %+v, want unverifiable", got)
	}
}
