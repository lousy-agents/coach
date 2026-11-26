package evalcensus

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// AppendToLedger appends whole JSON lines to the ledger at path under the
// ledger's lock, so concurrent writers append one at a time. lines runs
// under the lock and returns the bytes to append, or an error that refuses
// the append. A ledger whose last line lacks a newline gets one first, and a
// failed or short write truncates the ledger back to its length before the
// append, so no torn line is ever left for a reader to refuse.
func AppendToLedger(path string, lines func() ([]byte, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	unlock, err := lockLedger(path)
	if err != nil {
		return err
	}
	defer unlock()
	return appendLocked(path, lines)
}

// appendLocked builds and appends lines to the ledger at path, whose lock
// the caller holds.
func appendLocked(path string, lines func() ([]byte, error)) error {
	data, err := lines()
	if err != nil {
		return err
	}
	return appendBytes(path, data)
}

func appendBytes(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return errors.Join(err, file.Close())
	}
	if endsMidLine(file) {
		data = append([]byte{'\n'}, data...)
	}
	if _, err := writeLedger(file, data); err != nil {
		return errors.Join(err, file.Truncate(size), file.Close())
	}
	return file.Close()
}

// writeLedger writes an append's bytes to the open ledger.
var writeLedger = func(file *os.File, data []byte) (int, error) {
	return file.Write(data)
}
