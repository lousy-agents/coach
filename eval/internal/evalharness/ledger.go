package evalharness

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed ledger_line.schema.json
var ledgerLineSchemaJSON []byte

const ledgerLineSchemaName = "ledger_line.schema.json"

var compiledLedgerLineSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	runDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(runSchemaJSON))
	if err != nil {
		return nil, err
	}
	ledgerDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(ledgerLineSchemaJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(runSchemaName, runDoc); err != nil {
		return nil, err
	}
	if err := compiler.AddResource(ledgerLineSchemaName, ledgerDoc); err != nil {
		return nil, err
	}
	return compiler.Compile(ledgerLineSchemaName)
})

// LedgerLine is one validated line of the run ledger. Run is the full run record, which is
// self-contained: the trend never reads a local run.json.
type LedgerLine struct {
	Kind     string
	Recorded string
	Cell     string
	Run      map[string]any
}

// LoadLedger reads the JSON Lines ledger at path and validates every non-blank line against the
// committed ledger line schema.
func LoadLedger(path string) ([]LedgerLine, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("ledger not found: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read ledger %s: %w", path, err)
	}
	schema, err := compiledLedgerLineSchema()
	if err != nil {
		return nil, fmt.Errorf("compile ledger line schema: %w", err)
	}
	var lines []LedgerLine
	for index, text := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(text) == "" {
			continue
		}
		line, err := parseLedgerLine(schema, text)
		if err != nil {
			return nil, fmt.Errorf("ledger %s line %d: %w", path, index+1, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func parseLedgerLine(schema *jsonschema.Schema, text string) (LedgerLine, error) {
	instance, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
	if err != nil {
		return LedgerLine{}, fmt.Errorf("not JSON: %w", err)
	}
	if err := schema.Validate(instance); err != nil {
		return LedgerLine{}, err
	}
	object, ok := instance.(map[string]any)
	if !ok {
		return LedgerLine{}, errors.New("not a JSON object")
	}
	run, _ := object["run"].(map[string]any)
	return LedgerLine{
		Kind:     fmt.Sprint(object["kind"]),
		Recorded: fmt.Sprint(object["recorded"]),
		Cell:     fmt.Sprint(object["cell"]),
		Run:      run,
	}, nil
}
