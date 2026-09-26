package main

import "testing"

func TestAnalyzerEnvironSamplerCountsPIDReuseAsDistinctInvocations(t *testing.T) {
	s := newIdleAnalyzerEnvironSampler()
	s.record(7, "100", "PATH=/a\n", true)
	s.record(7, "200", "PATH=/b\n", true)

	if got := s.invocations(); got != 2 {
		t.Fatalf("invocations() = %d, want 2: two sequential analyzer children that reuse a PID must still count as two invocations (got environs %#v)", got, s.byPID)
	}
}

func TestAnalyzerEnvironSamplerCountsChildWithEmptyEnviron(t *testing.T) {
	s := newIdleAnalyzerEnvironSampler()
	s.record(7, "100", "", false)

	if got := s.invocations(); got != 1 {
		t.Fatalf("invocations() = %d, want 1: a cmdline-matched child whose environ read is empty (process already torn down) must still count as an invocation", got)
	}
	if len(s.byPID) != 0 {
		t.Fatalf("byPID = %#v, want empty so PATH assertions do not treat a torn-down environ as a real sample", s.byPID)
	}
}

func TestAnalyzerEnvironSamplerDoesNotDoubleCountSamePIDWithAndWithoutStartTime(t *testing.T) {
	s := newIdleAnalyzerEnvironSampler()
	s.record(7, "100", "PATH=/a\n", true)
	s.record(7, "", "", false)

	if got := s.invocations(); got != 1 {
		t.Fatalf("invocations() = %d, want 1: a live sample plus a torn-down sample of the same PID must be one invocation", got)
	}

	s = newIdleAnalyzerEnvironSampler()
	s.record(7, "", "", false)
	s.record(7, "100", "PATH=/a\n", true)

	if got := s.invocations(); got != 1 {
		t.Fatalf("invocations() = %d, want 1: a torn-down sample followed by a starttime sample of the same PID must be one invocation", got)
	}
}
