package evalharness

import (
	"errors"
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// maxAttempts bounds the replacements in one cell: the attempts are the seed and the nine seeds after it.
const maxAttempts = 10

var errSlotExhausted = errors.New("every attempt in the cell was rejected")

type generateOutput struct {
	ProgramID    string       `json:"program_id"`
	Path         string       `json:"path"`
	OracleDir    string       `json:"oracle_dir"`
	Cache        string       `json:"cache"`
	Seed         *int64       `json:"seed,omitempty"`
	Replacements int          `json:"replacements"`
	Cell         evalgen.Cell `json:"cell"`
}

// generateRun is one `generate` invocation: its store, its faults and the tally of its attempts.
type generateRun struct {
	request GenerateRequest
	store   evalgen.Store
	faults  faults
	tools   evalgen.TypeScriptTools
	tally   rejectionTally
}

func newGenerateRun(request GenerateRequest) (*generateRun, error) {
	tools, err := toolsFor(request.Language)
	if err != nil {
		return nil, err
	}
	injected, err := faultsFromEnvironment()
	if err != nil {
		return nil, err
	}
	store, err := storeFor(request)
	if err != nil {
		return nil, err
	}
	return &generateRun{request: request, store: store, faults: injected, tools: tools}, nil
}

func (r *generateRun) produceAll() ([]generateOutput, error) {
	outputs := make([]generateOutput, 0, len(r.request.Cells))
	for _, cell := range r.request.Cells {
		output, err := r.produce(cell)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func (r *generateRun) produce(cell evalgen.Cell) (generateOutput, error) {
	key := evalgen.Key(r.request.Language, cell, r.request.Seed, r.request.Panel)
	location, manifest, intact := r.store.Lookup(key)
	cache := "hit"
	if r.request.NoCache || !intact {
		cache = "miss"
		var err error
		if location, manifest, err = r.generateAndSave(cell, key); err != nil {
			return generateOutput{}, err
		}
	}
	if r.request.Out != "" {
		if err := copyProgram(location.Program, filepath.Join(r.request.Out, manifest.Program.ProgramID)); err != nil {
			return generateOutput{}, err
		}
	}
	return r.output(cell, location, manifest, cache), nil
}

func (r *generateRun) output(cell evalgen.Cell, location evalgen.Location, manifest evalgen.Manifest, cache string) generateOutput {
	output := generateOutput{
		ProgramID: manifest.Program.ProgramID, Path: location.Program, OracleDir: location.Oracle,
		Cache: cache, Replacements: manifest.Program.Replacements, Cell: cell,
	}
	if r.request.Panel == PanelTuning {
		seed := r.request.Seed + int64(manifest.Program.Replacements)
		output.Seed = &seed
	}
	return output
}
