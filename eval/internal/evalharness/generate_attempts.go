package evalharness

import (
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// generateAndSave replaces a rejected program with the next seed in the same cell, up to maxAttempts.
func (r *generateRun) generateAndSave(cell evalgen.Cell, key string) (evalgen.Location, evalgen.Manifest, error) {
	if r.request.Language == evalgen.LanguageTypeScript {
		return r.generateAndSaveOnce(cell, key)
	}
	for replacement := 0; replacement < maxAttempts; replacement++ {
		seed := r.request.Seed + int64(replacement)
		program, err := generateProgram(r.request.Language, cell, seed)
		if err != nil {
			return evalgen.Location{}, evalgen.Manifest{}, err
		}
		rejection, oracleFields, err := r.validate(program, seed)
		if err != nil {
			return evalgen.Location{}, evalgen.Manifest{}, err
		}
		if rejection != "" {
			r.tally.reject(rejection)
			continue
		}
		outcome, err := r.judgeMutation(program)
		if err != nil {
			return evalgen.Location{}, evalgen.Manifest{}, err
		}
		if outcome.rejectedAt != "" {
			r.tally.reject(outcome.rejectedAt)
			continue
		}
		r.tally.accept()
		program = withValidation(withValidation(program, oracleFields), outcome.fields)
		program.Manifest.Program.Panel = r.request.Panel
		program.Manifest.Program.Replacements = replacement
		program = withBaseFindings(r.coach, program)
		location, err := r.store.Save(key, program)
		return location, program.Manifest, err
	}
	return evalgen.Location{}, evalgen.Manifest{}, errSlotExhausted
}
