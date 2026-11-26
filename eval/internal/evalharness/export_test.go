package evalharness

import "github.com/lousy-agents/coach/eval/internal/evalgen"

// JudgeMutation runs the mutation gate on a program with a cache in cacheDirectory and returns the step that rejected
// it, or "" when the gate passed.
func JudgeMutation(gremlins Gremlins, cacheDirectory string, program evalgen.Program) (string, error) {
	outcome, err := judgeMutation(gremlins, mutationCache{directory: cacheDirectory}, program)
	return outcome.rejectedAt, err
}
