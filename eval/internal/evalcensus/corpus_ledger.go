package evalcensus

import (
	"fmt"
	"path/filepath"
)

// AppendCorpusRecords appends records to the corpus's ledger,
// <censusRoot>/<corpusID>/records.jsonl. The corpus_id names a directory, so
// it must be a path segment, and every record must belong to that corpus.
func AppendCorpusRecords(censusRoot, corpusID string, records []Record) error {
	if err := corpusRecordsProblem(corpusID, records); err != nil {
		return err
	}
	return AppendRecords(corpusRecordsPath(censusRoot, corpusID), records)
}

func corpusRecordsProblem(corpusID string, records []Record) error {
	if !corpusIDPattern.MatchString(corpusID) {
		return fmt.Errorf("corpus_id %q is not a lowercase path segment", corpusID)
	}
	for _, record := range records {
		if record.Target.CorpusID != corpusID {
			return fmt.Errorf("%w %q: target.corpus_id %q is not the ledger's corpus %q", ErrInvalidRecord, record.RecordID, record.Target.CorpusID, corpusID)
		}
	}
	return nil
}

func corpusRecordsPath(censusRoot, corpusID string) string {
	return filepath.Join(censusRoot, corpusID, "records.jsonl")
}
