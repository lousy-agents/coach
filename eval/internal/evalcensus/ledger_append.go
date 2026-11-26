package evalcensus

import (
	"bytes"
	"encoding/json"
)

// appendRecords validates the whole batch against itself and the ledger, then
// appends it. A batch with any invalid record writes nothing, and no record
// already in the ledger is ever rewritten.
func appendRecords(path string, records []Record) error {
	return appendToLedger(path, checkedRecordLines(path, records))
}

// checkedRecordLines validates records against the ledger at path, which
// the caller has locked, and returns them as JSON lines.
func checkedRecordLines(path string, records []Record) func() ([]byte, error) {
	return func() ([]byte, error) {
		existing, err := LoadRecords(path)
		if err != nil {
			return nil, err
		}
		if err := validateBatch(existing, records); err != nil {
			return nil, err
		}
		return recordLines(records)
	}
}

func recordLines(records []Record) ([]byte, error) {
	var lines bytes.Buffer
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		lines.Write(append(line, '\n'))
	}
	return lines.Bytes(), nil
}
