package evalharness

import (
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// pinnedGremlins is the tool the gate runs, or the zero value when the run skips the gate or generates a language
// the gate does not cover. It is resolved before anything is generated, so a missing tool is a missing input.
func pinnedGremlins(request GenerateRequest) (Gremlins, error) {
	if request.NoMutation || request.Language != evalgen.LanguageGo {
		return Gremlins{}, nil
	}
	if binary, version := gremlinsOverride(); binary != "" {
		return Gremlins{Binary: binary, Version: version}, nil
	}
	module, err := findEvalModule()
	if err != nil {
		return Gremlins{}, err
	}
	return ResolveGremlins(module, nil)
}

// judgeMutation runs the gate for a program that passed the other steps, unless the run skipped it. A skipped
// gate is recorded as skipped.
func (r *generateRun) judgeMutation(program evalgen.Program) (mutationOutcome, error) {
	if r.request.NoMutation {
		return mutationOutcome{fields: map[string]any{"mutation_checked": false}}, nil
	}
	if r.gremlins.Binary == "" {
		return mutationOutcome{}, nil
	}
	cache := mutationCache{directory: filepath.Join(filepath.Dir(r.store.Root), "mutation")}
	return judgeMutation(r.gremlins, cache, program)
}

// withValidation returns the program with fields added to its manifest's validation record.
func withValidation(program evalgen.Program, fields map[string]any) evalgen.Program {
	validation := make(map[string]any, len(program.Manifest.Validation)+len(fields))
	for key, value := range program.Manifest.Validation {
		validation[key] = value
	}
	for key, value := range fields {
		validation[key] = value
	}
	program.Manifest.Validation = validation
	return program
}
