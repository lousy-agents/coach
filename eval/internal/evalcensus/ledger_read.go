package evalcensus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// LoadRecords reads the append-only JSON Lines ledger at path, validating
// every record. A missing ledger holds no records.
func LoadRecords(path string) ([]Record, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read census ledger %s: %w", path, err)
	}
	var records []Record
	for index, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		record, err := decodeRecord(line)
		if err != nil {
			return nil, fmt.Errorf("census ledger %s line %d: %w", path, index+1, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func decodeRecord(line string) (Record, error) {
	var record Record
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	return record, validateRecord(record)
}
