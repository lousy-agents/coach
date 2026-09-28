package codesignalcli

import (
	"fmt"

	"reflect"
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

// TestAggregateReadinessOrdersNextActionsPolicyBeforeCompiler proves AC-SET-13
// directly: when a policy gap and a compiler gap exist simultaneously,
// author_policy precedes prepare_compiler in next_actions. Exercised
// directly against aggregateReadiness, rather than through resolveCompiler,
// so the ordering assertion does not depend on constructing a real
// typescript_compiler_missing fixture.
func TestAggregateReadinessOrdersNextActionsPolicyBeforeCompiler(t *testing.T) {
	checks := ReadinessChecks{
		Policy:   ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing},
		Compiler: ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
	}
	_, _, nextActions, _ := aggregateReadiness(checks, false, nil)
	want := []ReadinessNextAction{
		{Kind: "author_policy", Executable: false},
		{Kind: "prepare_compiler", Executable: true, Supported: []string{"7.0.2"}},
	}
	if len(nextActions) != len(want) {
		t.Fatalf("nextActions = %#v, want %#v", nextActions, want)
	}
	for i, action := range want {
		if !reflect.DeepEqual(nextActions[i], action) {
			t.Fatalf("nextActions[%d] = %#v, want %#v (full: %#v)", i, nextActions[i], action, nextActions)
		}
	}
}

func TestAggregateReadinessEmitsCompilerDeclarationMismatchWarningShape(t *testing.T) {
	checks := ReadinessChecks{
		Compiler: ReadinessCheck{
			State:             ReadinessPass,
			Code:              WarnCompilerDeclarationMismatch,
			Version:           "7.0.2",
			DeclarationOrigin: compilerDeclarationOriginManifest,
			DeclarationMismatches: []ReadinessDeclarationMismatch{
				{Root: ".", Declared: "5.4.0"},
			},
		},
	}
	_, _, _, warnings := aggregateReadiness(checks, false, nil)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want exactly one entry", warnings)
	}
	want := ReadinessWarning{Code: WarnCompilerDeclarationMismatch, DeclaredVersion: "5.4.0", FoundVersion: "7.0.2", DeclarationOrigin: compilerDeclarationOriginManifest, Root: "."}
	if warnings[0] != want {
		t.Fatalf("warnings[0] = %#v, want %#v", warnings[0], want)
	}
}
