package tstoolchain

import (
	"path/filepath"
	"strconv"
	"testing"
)

func TestNodeVersionConstantsMatchDeclaredPins(t *testing.T) {
	root := coachRepoRoot(t)

	engines := packageJSONEnginesNode(t, filepath.Join(root, "js", "semantics", "package.json"))
	lockEngines := packageLockRootEnginesNode(t, filepath.Join(root, "js", "semantics", "package-lock.json"))
	if engines != lockEngines {
		t.Fatalf("js/semantics package.json engines.node = %q, package-lock.json root engines.node = %q", engines, lockEngines)
	}
	wantEnginesNode := wantEnginesNodeString(SupportedNodeMajors)
	if engines != wantEnginesNode {
		t.Fatalf("js/semantics engines.node = %q, want %q (must restate SupportedNodeMajors %v)", engines, wantEnginesNode, SupportedNodeMajors)
	}

	majors, err := nodeMajorsFromEnginesRangeUnion(engines)
	if err != nil {
		t.Fatalf("parse engines %q: %v", engines, err)
	}
	assertSameNodeMajorSet(t, "js/semantics engines.node", majors, SupportedNodeMajors)

	tested, err := testedNodeMajorFromMise(readFileT(t, filepath.Join(root, "mise.toml")))
	if err != nil {
		t.Fatalf("parse mise.toml node pin: %v", err)
	}
	if !NodeMajorSupported(tested) {
		t.Fatalf("mise.toml [tools].node pin %d is not in SupportedNodeMajors %v", tested, SupportedNodeMajors)
	}
}

// TestNodeMajorSupportedMatchesAnalysisGate binds CheckNode's
// set-membership predicate (NodeMajorSupported, backed by
// SupportedNodeMajors) to tsRuntime's independent AnalysisNodeMajorAllowed
// (project_ts_runtime.go): the two are frozen to agree on {24, 26} today,
// but nothing else ties them together, so a change to one that silently
// diverges from the other would make the readiness verdict and the
// analysis gate disagree on the same host Node major. This test must turn
// red the moment either one changes without the other.
func TestNodeMajorSupportedMatchesAnalysisGate(t *testing.T) {
	for major := 20; major <= 30; major++ {
		t.Run(strconv.Itoa(major), func(t *testing.T) {
			checkNodeMajorSupportedMatchesAnalysisGate(t, major)
		})
	}
}

func checkNodeMajorSupportedMatchesAnalysisGate(t *testing.T, major int) {
	if got, want := NodeMajorSupported(major), AnalysisNodeMajorAllowed(major); got != want {
		t.Fatalf("nodeMajorSupported(%d) = %t, analysisNodeMajorAllowed(%d) = %t: readiness and analysis gates disagree", major, got, major, want)
	}
}
