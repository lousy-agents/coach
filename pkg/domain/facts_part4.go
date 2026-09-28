package domain

import (
	"sort"
)

func canonicalReachabilityFacts(in []ReachabilityFact) []ReachabilityFact {
	if len(in) == 0 {
		return in
	}
	out := append([]ReachabilityFact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Sink != out[j].Sink {
			return out[i].Sink < out[j].Sink
		}
		return out[i].ID < out[j].ID
	})
	return out
}
func CanonicalCoverage(in Coverage) Coverage {
	if len(in.Diagnostics) == 0 {
		return in
	}
	out := in
	diagnostics := append([]Diagnostic(nil), in.Diagnostics...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	out.Diagnostics = diagnostics
	return out
}

// canonicalRootScopes assumes Root is unique per Model. A duplicate Root is
// unsupported input with unspecified relative order.
func canonicalRootScopes(in []RootScope) []RootScope {
	if len(in) == 0 {
		return in
	}
	out := append([]RootScope(nil), in...)
	for i := range out {
		out[i].AnalyzedPaths = sortedStrings(out[i].AnalyzedPaths)
		out[i].UnanalyzedPaths = sortedStrings(out[i].UnanalyzedPaths)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Root < out[j].Root
	})
	return out
}
func CanonicalCallFacts(in []CallFact) []CallFact {
	if len(in) == 0 {
		return in
	}
	out := append([]CallFact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}
