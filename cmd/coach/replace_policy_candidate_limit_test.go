package main

import (
	"fmt"
	"testing"
)

// The last name tried is the 99th: with every name before it taken, p.candidate-99.json is offered, and the first name past the
// limit is not. A limit that stopped one short would pass the test above, which fills every name.
func TestTheLastNameBesideThePolicyIsOfferedWhenAllBeforeItAreTaken(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "p.candidate.json")
	for attempt := 2; attempt < maxCandidateSuffix; attempt++ {
		touch(t, root, fmt.Sprintf("p.candidate-%d.json", attempt))
	}

	got, found := freeCandidateBeside(root, "p.json")

	if want := fmt.Sprintf("p.candidate-%d.json", maxCandidateSuffix); !found || got != want {
		t.Errorf("freeCandidateBeside = %q, %v; want %q, true: it is the last name the limit allows", got, found, want)
	}
}
