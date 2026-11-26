package evalcensus

import (
	"bytes"
	"encoding/json"
)

// AppendRecords validates the whole batch against itself and the ledger, then
// appends it. A batch with any invalid record writes nothing, and no record
// already in the ledger is ever rewritten.
func AppendRecords(path string, records []Record) error {
	return AppendToLedger(path, func() ([]byte, error) {
		existing, err := LoadRecords(path)
		if err != nil {
			return nil, err
		}
		if err := validateBatch(existing, records); err != nil {
			return nil, err
		}
		return recordLines(records)
	})
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
