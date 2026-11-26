package evalharness

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// validLedgerLine marshals line and checks it against the ledger line
// schema.
func validLedgerLine(line any) ([]byte, error) {
	data, err := json.Marshal(line)
	if err != nil {
		return nil, err
	}
	schema, err := compiledLedgerLineSchema()
	if err != nil {
		return nil, fmt.Errorf("compile ledger line schema: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := schema.Validate(instance); err != nil {
		return nil, fmt.Errorf("ledger line does not fit its schema: %w", err)
	}
	return data, nil
}
