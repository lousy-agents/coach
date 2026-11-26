package evalharness

import (
	"maps"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// oracleTree is one oracle of a program as validation builds it: the first feature's oracle, and the follow-on's,
// which replaces the first feature's hidden tests with its own.
type oracleTree struct {
	name        string
	files       map[string][]byte
	hiddenTests map[string][]byte
}

func oracleTrees(program evalgen.Program) []oracleTree {
	first := maps.Clone(program.Tree)
	maps.Copy(first, program.Solution)
	maps.Copy(first, program.Tests)

	followOn := maps.Clone(program.Tree)
	maps.Copy(followOn, program.Solution)
	maps.Copy(followOn, program.FollowOnTests)
	maps.Copy(followOn, program.FollowOnSolution)

	return []oracleTree{
		{name: "first", files: first, hiddenTests: program.Tests},
		{name: "follow-on", files: followOn, hiddenTests: program.FollowOnTests},
	}
}
