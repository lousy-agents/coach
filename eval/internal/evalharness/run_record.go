package evalharness

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed run.schema.json
var runSchemaJSON []byte

const runSchemaName = "run.schema.json"

var compiledRunSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(runSchemaJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(runSchemaName, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(runSchemaName)
})

// LoadRunRecord reads the run record of runID under stateRoot and validates it against the
// committed run schema, version 1.
func LoadRunRecord(stateRoot, runID string) (map[string]any, error) {
	path := filepath.Join(stateRoot, "coach-eval", runID, "run.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no run record for %s (expected %s)", runID, path)
	}
	if err != nil {
		return nil, fmt.Errorf("read run record for %s: %w", runID, err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("run record for %s is not JSON: %w", runID, err)
	}
	schema, err := compiledRunSchema()
	if err != nil {
		return nil, fmt.Errorf("compile run schema: %w", err)
	}
	if err := schema.Validate(instance); err != nil {
		return nil, fmt.Errorf("run record for %s fails the run schema: %w", runID, err)
	}
	record, ok := instance.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("run record for %s is not a JSON object", runID)
	}
	return record, nil
}
