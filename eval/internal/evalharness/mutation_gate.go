package evalharness

import (
	"maps"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// mutationRun is the two suites the gate judges (design §6, step 6): the PASS_TO_PASS suite on the unmutated
// program, and the FAIL_TO_PASS suite on the first feature's oracle.
type mutationRun struct {
	suite string
	tree  map[string][]byte
}

func mutationRuns(program evalgen.Program) []mutationRun {
	oracle := maps.Clone(program.Tree)
	maps.Copy(oracle, program.Solution)
	maps.Copy(oracle, program.Tests)
	return []mutationRun{{"pass_to_pass", program.Tree}, {"fail_to_pass", oracle}}
}

// routeDirectories are the packages of the api surface's symbols: the entry's and each hop's.
func routeDirectories(program evalgen.Program) []string {
	var directories []string
	prefix := moduleNameOf(program) + "/"
	for _, symbol := range program.Manifest.Template.APISurface {
		qualified := strings.TrimPrefix(symbol.Symbol, prefix)
		directories = append(directories, qualified[:strings.LastIndex(qualified, ".")])
	}
	return directories
}

// mutationOutcome is the gate's result for one program: the step that rejected it, if any, and what to record.
type mutationOutcome struct {
	rejectedAt string
	fields     map[string]any
}

// judgeMutation runs the gate and records the verdicts. A run that passes vacuously (no valid mutant, which is
// every base program until operators add branches) is recorded with a null score.
func judgeMutation(gremlins Gremlins, cache mutationCache, program evalgen.Program) (mutationOutcome, error) {
	scope, err := MutationScope(program)
	if err != nil {
		return mutationOutcome{}, err
	}
	runs := map[string]any{}
	var scores, shares []float64
	for _, run := range mutationRuns(program) {
		report, err := cache.report(gremlins, run.tree)
		if err != nil {
			return mutationOutcome{}, err
		}
		counts, err := ParseMutationReport(report, scope)
		if err != nil {
			return mutationOutcome{}, err
		}
		verdict := counts.Verdict()
		if !verdict.Pass {
			return mutationOutcome{rejectedAt: evalgen.StepMutation}, nil
		}
		runs[run.suite] = verdictFields(verdict)
		if verdict.Score != nil {
			scores, shares = append(scores, *verdict.Score), append(shares, *verdict.NotCoveredShare)
		}
	}
	return mutationOutcome{fields: map[string]any{
		"mutation_checked": true, "mutation_runs": runs,
		"mutation_score": lowest(scores), "not_covered_share": highest(shares),
	}}, nil
}

func verdictFields(verdict MutationVerdict) map[string]any {
	return map[string]any{"valid": verdict.Valid, "score": verdict.Score, "not_covered_share": verdict.NotCoveredShare}
}

func lowest(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	lowest := values[0]
	for _, value := range values {
		lowest = min(lowest, value)
	}
	return lowest
}

func highest(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	highest := values[0]
	for _, value := range values {
		highest = max(highest, value)
	}
	return highest
}
