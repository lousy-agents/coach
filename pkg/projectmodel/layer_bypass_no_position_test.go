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
