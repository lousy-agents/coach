package evalharness

import (
	"fmt"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// generateAndSaveOnce is the TypeScript path: one attempt, and a failed validation is an error. Its graders,
// and so its rejection and replacement, arrive with the TypeScript validation slice.
func (r *generateRun) generateAndSaveOnce(cell evalgen.Cell, key string) (evalgen.Location, evalgen.Manifest, error) {
	program, err := generateProgram(r.request.Language, cell, r.request.Seed)
	if err != nil {
		return evalgen.Location{}, evalgen.Manifest{}, err
	}
	program.Manifest.Program.Panel = r.request.Panel
	program.Manifest.Program.Profile = r.request.Profile
	if err := validateProgram(program, r.tools); err != nil {
		return evalgen.Location{}, evalgen.Manifest{}, fmt.Errorf("the generated program failed validation: %w", err)
	}
	program = withBaseFindings(r.coach, program)
	location, err := r.store.Save(key, program)
	return location, program.Manifest, err
}
