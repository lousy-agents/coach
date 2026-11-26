package evalcensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// AppendRecords validates the whole batch against itself and the ledger, then
// appends it. A batch with any invalid record writes nothing, and no record
// already in the ledger is ever rewritten.
func AppendRecords(path string, records []Record) error {
	existing, err := LoadRecords(path)
	if err != nil {
		return err
	}
	if err := validateBatch(existing, records); err != nil {
		return err
	}
	return appendLines(path, records)
}

func validateBatch(existing, batch []Record) error {
	known := make(map[string]bool, len(existing)+len(batch))
	for _, record := range existing {
		known[record.RecordID] = true
	}
	for _, record := range batch {
		if err := validateRecord(record); err != nil {
			return err
		}
		if err := checkLinks(known, record); err != nil {
			return err
		}
		known[record.RecordID] = true
	}
	return nil
}

func checkLinks(known map[string]bool, record Record) error {
	if known[record.RecordID] {
		return fmt.Errorf("%w %q: record_id is already in the ledger", ErrInvalidRecord, record.RecordID)
	}
	if record.Supersedes != nil && !known[*record.Supersedes] {
		return fmt.Errorf("%w %q: supersedes %q, which is not an earlier record", ErrInvalidRecord, record.RecordID, *record.Supersedes)
	}
	return nil
}

func appendLines(path string, records []Record) error {
	var lines bytes.Buffer
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			return err
		}
		lines.Write(append(line, '\n'))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(lines.Bytes()); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
