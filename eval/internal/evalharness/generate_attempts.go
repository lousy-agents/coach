package evalharness

import (
	"errors"
	"os"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// generateAndSave replaces a rejected program with the next seed in the same cell, up to maxAttempts.
func (r *generateRun) generateAndSave(cell evalgen.Cell, key string) (evalgen.Location, evalgen.Manifest, error) {
	for replacement := 0; replacement < maxAttempts; replacement++ {
		seed := r.request.Seed + int64(replacement)
		program, err := evalgen.Generate(cell, seed)
		if err != nil {
			return evalgen.Location{}, evalgen.Manifest{}, err
		}
		rejection, err := r.validate(program, seed)
		if err != nil {
			return evalgen.Location{}, evalgen.Manifest{}, err
		}
		if rejection != "" {
			r.tally.reject(rejection)
			continue
		}
		r.tally.accept()
		program.Manifest.Program.Panel = r.request.Panel
		program.Manifest.Program.Replacements = replacement
		location, err := r.store.Save(key, program)
		return location, program.Manifest, err
	}
	return evalgen.Location{}, evalgen.Manifest{}, errSlotExhausted
}

// validate returns the step that rejected the program, or "" when every step passed. An error is a tool that
// could not run, which is not a rejection.
func (r *generateRun) validate(program evalgen.Program, seed int64) (string, error) {
	for _, step := range evalgen.ValidationSteps {
		if r.faults.rejectsAt(step, seed) {
			return step, nil
		}
	}
	if err := evalgen.Validate(program); err != nil {
		return rejectionStep(err)
	}
	return gradeProgram(program)
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
