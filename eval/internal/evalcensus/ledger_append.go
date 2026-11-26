package evalcensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// AppendRecords validates the whole batch against itself and the ledger, then
// appends it. A batch with any invalid record writes nothing, and no record
// already in the ledger is ever rewritten.
func AppendRecords(path string, records []Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	unlock, err := lockLedger(path)
	if err != nil {
		return err
	}
	defer unlock()
	existing, err := LoadRecords(path)
	if err != nil {
		return err
	}
	if err := validateBatch(existing, records); err != nil {
		return err
	}
	return appendLines(path, records)
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
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return err
	}
	if endsMidLine(file) {
		lines = *bytes.NewBuffer(append([]byte{'\n'}, lines.Bytes()...))
	}
	if _, err := writeLedger(file, lines.Bytes()); err != nil {
		return errors.Join(err, file.Truncate(size), file.Close())
	}
	return file.Close()
}

// writeLedger writes an append's bytes to the open ledger.
var writeLedger = func(file *os.File, data []byte) (int, error) {
	return file.Write(data)
}
