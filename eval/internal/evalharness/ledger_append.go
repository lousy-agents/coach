package evalharness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// AppendLedgerLine validates line against the ledger line schema and appends
// it to the ledger at path as one JSON line. A ledger whose last line lacks
// a newline gets one first, so the new line never joins it.
func AppendLedgerLine(path string, line any) error {
	data, err := validLedgerLine(line)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return errors.Join(err, file.Close())
	}
	if size > 0 && !endsWithNewline(file, size) {
		data = append([]byte{'\n'}, data...)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
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

func endsWithNewline(file *os.File, size int64) bool {
	last := make([]byte, 1)
	_, err := file.ReadAt(last, size-1)
	return err == nil && last[0] == '\n'
}
