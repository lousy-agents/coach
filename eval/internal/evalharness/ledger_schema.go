package evalharness

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
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

// parseLedgerLine decodes one ledger line and validates it against the committed ledger line schema.
func parseLedgerLine(text string) (LedgerLine, error) {
	schema, err := compiledLedgerLineSchema()
	if err != nil {
		return LedgerLine{}, fmt.Errorf("compile ledger line schema: %w", err)
	}
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
