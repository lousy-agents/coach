package evalcensus

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// AppendRun writes a run's records and its ledger line as one unit, so a
// re-run never duplicates records: it holds the run ledger's lock across
// both writes, and when the ledger line fails it cuts the corpus's records
// back to what they were before the run.
func AppendRun(ledgerPath string, line []byte, censusRoot, corpusID string, records []Record) error {
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return fmt.Errorf("write census ledger line: %w", err)
	}
	unlock, err := lockLedger(ledgerPath)
	if err != nil {
		return err
	}
	defer unlock()
	recordsPath := corpusRecordsPath(censusRoot, corpusID)
	restore, err := snapshotLength(recordsPath)
	if err != nil {
		return err
	}
	afterRecordsSnapshot()
	if err := AppendCorpusRecords(censusRoot, corpusID, records); err != nil {
		return fmt.Errorf("write census records: %w", err)
	}
	if err := appendBytes(ledgerPath, line); err != nil {
		return errors.Join(fmt.Errorf("write census ledger line: %w", err), restore())
	}
	return nil
}

// afterRecordsSnapshot runs between noting the records' length and writing
// them; specs replace it to race another writer there.
var afterRecordsSnapshot = func() {}

// snapshotLength returns a function that puts the file at path back to its
// current length, or removes it when it does not exist yet.
func snapshotLength(path string) (func() error, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return func() error { return os.Remove(path) }, nil
	}
	if err != nil {
		return nil, err
	}
	return func() error { return os.Truncate(path, info.Size()) }, nil
}
