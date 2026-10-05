package tstoolchain

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestMiseToolVersionToken(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "real mise output", input: "2026.9.5 linux-x64 (2026-09-10)", want: "2026.9.5", wantOK: true},
		{name: "bare version, no platform suffix", input: "2026.9.5", want: "2026.9.5", wantOK: true},
		{name: "leading/trailing whitespace", input: "  2026.9.5 linux-x64  ", want: "2026.9.5", wantOK: true},
		{name: "empty", input: "", want: "", wantOK: false},
		{name: "all whitespace", input: "   ", want: "", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectTsCompilerMiseVersionTest_22(t, tc)
		})
	}
}

func stubProbeMiseToolVersion(t *testing.T, version string, ok bool) {
	t.Helper()
	original := ProbeMiseToolVersion
	t.Cleanup(func() { ProbeMiseToolVersion = original })
	ProbeMiseToolVersion = func(context.Context) (string, bool) {
		return version, ok
	}
}

func TestEvaluateMiseToolVersionReadiness(t *testing.T) {
	t.Run("in-row version is ready", func(t *testing.T) {
		body_projectTsCompilerMiseVersionTest_inRowVersionIsReady_41(t)
	})

	t.Run("out-of-row version is unsupported", func(t *testing.T) {
		body_projectTsCompilerMiseVersionTest_outOfRowVersionIsUnsupported_49(t)
	})

	t.Run("failed probe is unverifiable", func(t *testing.T) {
		body_projectTsCompilerMiseVersionTest_failedProbeIsUnverifiable_57(t)
	})

	t.Run("empty successful probe output is unverifiable, not unsupported", func(t *testing.T) {
		body_projectTsCompilerMiseVersionTest_emptySuccessfulProbeOutputIsUnverifiableNotUnsup_65(t)
	})
}

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
