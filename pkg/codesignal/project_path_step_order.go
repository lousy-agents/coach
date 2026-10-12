package codesignal

import (
	"strings"
)

func comparePathSteps(a, b ProjectPathStep) int {
	if c := strings.Compare(a.NodeID, b.NodeID); c != 0 {
		return c
	}
	if c := strings.Compare(a.DisplayName, b.DisplayName); c != 0 {
		return c
	}
	if c := strings.Compare(a.Resolution, b.Resolution); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Confidence), string(b.Confidence)); c != 0 {
		return c
	}
	n := len(a.SourceLocations)
	if len(b.SourceLocations) < n {
		n = len(b.SourceLocations)
	}
	for i := 0; i < n; i++ {
		la, lb := a.SourceLocations[i], b.SourceLocations[i]
		if c := strings.Compare(la.Path, lb.Path); c != 0 {
			return c
		}
		if c := compareLocationValue(la.Location, lb.Location); c != 0 {
			return c
		}
	}
	switch {
	case len(a.SourceLocations) < len(b.SourceLocations):
		return -1
	case len(a.SourceLocations) > len(b.SourceLocations):
		return 1
	default:
		return 0
	}
}
