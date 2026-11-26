package evalharness

import (
	"errors"
	"os"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// validate returns the step that rejected the program, or "" when every step passed, with what the oracle checks
// record in the manifest. An error is a tool that could not run, which is not a rejection.
func (r *generateRun) validate(program evalgen.Program, seed int64) (string, map[string]any, error) {
	for _, step := range evalgen.ValidationSteps {
		if r.faults.rejectsAt(step, seed, program.Cell.Index()) {
			return step, nil, nil
		}
	}
	if err := evalgen.Validate(program); err != nil {
		step, err := rejectionStep(err)
		return step, nil, err
	}
	if step, err := gradeProgram(program); step != "" || err != nil {
		return step, nil, err
	}
	grades, err := GradeOracles(program)
	if err != nil {
		step, err := rejectionStep(err)
		return step, nil, err
	}
	return "", oracleFields(grades), nil
}

// oracleFields is the validation record of the oracle checks. The native G1 is recorded under its own name so that
// it is never read as the nonce-channel G1 (#791).
func oracleFields(grades OracleGrades) map[string]any {
	return map[string]any{
		"g2_checked": true,
		"g1_native":  map[string]any{"first_oracle": grades.FirstOracleTests, "follow_on_oracle": grades.FollowOnOracleTests},
	}
}

func gradeProgram(program evalgen.Program) (string, error) {
	directory, err := os.MkdirTemp("", "coach-eval-grade")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	if err := evalgen.WriteTree(directory, program.Tree); err != nil {
		return "", err
	}
	if err := GradeBase(directory); err != nil {
		return rejectionStep(err)
	}
	return "", nil
}

func rejectionStep(err error) (string, error) {
	var stepError *evalgen.StepError
	if errors.As(err, &stepError) {
		return stepError.Step, nil
	}
	return "", err
}
