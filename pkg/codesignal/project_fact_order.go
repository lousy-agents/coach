package codesignal

import (
	"strings"
)

func compareProjectFacts(a, b ProjectFact) int {
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	if c := strings.Compare(a.SemanticKey, b.SemanticKey); c != 0 {
		return c
	}
	if c := strings.Compare(a.Evidence, b.Evidence); c != 0 {
		return c
	}
	if c := strings.Compare(a.Provenance.Producer, b.Provenance.Producer); c != 0 {
		return c
	}
	if c := strings.Compare(a.Provenance.FindingKind, b.Provenance.FindingKind); c != 0 {
		return c
	}
	if c := compareStringSlices(a.CoverageRefs, b.CoverageRefs); c != 0 {
		return c
	}
	return comparePathStepSlices(a.PathSteps, b.PathSteps)
}

func comparePathStepSlices(a, b []ProjectPathStep) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := comparePathSteps(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
