package evalharness

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// Exit codes of `coach-eval generate`.
const (
	GenerateOK           = 0
	GenerateFailed       = 1
	GenerateInputInvalid = 2
)

type generateOutput struct {
	ProgramID string `json:"program_id"`
	Path      string `json:"path"`
	OracleDir string `json:"oracle_dir"`
	Cache     string `json:"cache"`
}

// Generate emits one program, from the cache when it is intact, and prints one JSON line on stdout.
func Generate(args []string, stdout, stderr io.Writer) int {
	request, err := parseGenerate(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval generate: %v\n", err)
		return GenerateInputInvalid
	}
	store, err := storeFor(request)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval generate: %v\n", err)
		return GenerateInputInvalid
	}
	output, err := produce(store, request)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval generate: %v\n", err)
		return GenerateFailed
	}
	line, _ := json.Marshal(output)
	fmt.Fprintf(stdout, "%s\n", line)
	return GenerateOK
}

func produce(store evalgen.Store, request GenerateRequest) (generateOutput, error) {
	key := evalgen.Key(request.Cell, request.Seed, request.Panel)
	location, manifest, intact := store.Lookup(key)
	cache := "hit"
	if request.NoCache || !intact {
		cache = "miss"
		program, err := evalgen.Generate(request.Cell, request.Seed)
		if err != nil {
			return generateOutput{}, err
		}
		program.Manifest.Program.Panel = request.Panel
		if err := evalgen.Validate(program); err != nil {
			return generateOutput{}, fmt.Errorf("the generated program failed validation: %w", err)
		}
		if location, err = store.Save(key, program); err != nil {
			return generateOutput{}, err
		}
		manifest = program.Manifest
	}
	if request.Out != "" {
		if err := copyProgram(location.Program, filepath.Join(request.Out, manifest.Program.ProgramID)); err != nil {
			return generateOutput{}, err
		}
	}
	return generateOutput{ProgramID: manifest.Program.ProgramID, Path: location.Program, OracleDir: location.Oracle, Cache: cache}, nil
}
