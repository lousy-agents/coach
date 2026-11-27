package projectmodel

import (
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"

	"github.com/lousy-agents/coach/pkg/domain"
)

func TestGoFunctionWithoutSourcePositionRaisesTheNoSourcePositionCause(t *testing.T) {
	function := &ssa.Function{Prog: ssa.NewProgram(token.NewFileSet(), 0)}
	if _, ok := fnPosition(t.TempDir(), function); ok {
		t.Fatal("a function with no position resolved one")
	}
	path := []ReachabilityStep{{NodeID: "a function with no position"}, {NodeID: "the sink"}}
	if stepPathFullyClassified(path, map[string]layerBypassNodePosition{}) {
		t.Fatal("a path through a node with no position was treated as classified")
	}

	coverage := domain.CanonicalCoverage(domain.Coverage{Diagnostics: layerAmbiguity{}.diagnostics(nil, serviceLayer, true)})

	if got := coverage.Diagnostics[0].Message; !containsAll(got, "has no source position, so its layer could not be decided", "a limit of the analysis, not of the policy") {
		t.Fatalf("message does not name the cause the search raised: %q", got)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}

func TestGoSearchFlagsAPathThroughANodeWithoutSourcePosition(t *testing.T) {
	parents := map[string]string{"sink": "source"}
	search := &layerBypassSearch{}

	search.collectWitnesses("source", []string{"sink"}, parents, map[string]layerBypassNodePosition{}, "service", false)

	if !search.unclassifiedNodeSeen {
		t.Fatal("a path through a node with no position did not mark the search unclassified")
	}
	if len(search.witnesses) != 0 {
		t.Fatalf("a path through a node with no position produced a witness: %+v", search.witnesses)
	}
}

func TestGoSearchReportsAPathWhoseNodesAllHaveASourcePosition(t *testing.T) {
	parents := map[string]string{"sink": "source"}
	positions := map[string]layerBypassNodePosition{"source": {Dir: "pkg/handlers", File: "pkg/handlers/handler.go", Line: 1}}
	search := &layerBypassSearch{}

	search.collectWitnesses("source", []string{"sink"}, parents, positions, "service", false)

	if search.unclassifiedNodeSeen {
		t.Fatal("a fully positioned path marked the search unclassified")
	}
	if len(search.witnesses) != 1 {
		t.Fatalf("a fully positioned path produced %d witnesses, want 1", len(search.witnesses))
	}
}
