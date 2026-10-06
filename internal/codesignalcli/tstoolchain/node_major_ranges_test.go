package tstoolchain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// nodeMajorsFromEnginesRangeUnion parses a "^N || ^M ..." engines.node
// declaration into the set of Node majors it names. It understands only the
// caret-range-union syntax this repository's own manifests use (a discrete
// certified set, not a semver floor) -- not the full semver range grammar.
func nodeMajorsFromEnginesRangeUnion(engines string) ([]int, error) {
	terms := strings.Split(engines, "||")
	majors := make([]int, 0, len(terms))
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if !strings.HasPrefix(trimmed, "^") {
			return nil, fmt.Errorf("engines range %q: term %q is not %q-prefixed", engines, trimmed, "^")
		}
		majorPart, _, _ := strings.Cut(strings.TrimPrefix(trimmed, "^"), ".")
		major, err := strconv.Atoi(majorPart)
		if err != nil {
			return nil, fmt.Errorf("engines range %q: %w", engines, err)
		}
		majors = append(majors, major)
	}
	return majors, nil
}

func assertSameNodeMajorSet(t *testing.T, label string, got, want []int) {
	t.Helper()
	gotSorted := append([]int(nil), got...)
	wantSorted := append([]int(nil), want...)
	sort.Ints(gotSorted)
	sort.Ints(wantSorted)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("%s parses as %v, want %v", label, got, want)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("%s parses as %v, want %v", label, got, want)
		}
	}
}
