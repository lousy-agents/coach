package evalharness

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// AppendLedgerLine validates line against the ledger line schema and appends
// it to the ledger at path as one JSON line, with the records ledger's lock,
// newline repair and truncation on a failed write.
func AppendLedgerLine(path string, line any) error {
	data, err := validLedgerLine(line)
	if err != nil {
		return err
	}
	return evalcensus.AppendToLedger(path, func() ([]byte, error) { return append(data, '\n'), nil })
}

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
