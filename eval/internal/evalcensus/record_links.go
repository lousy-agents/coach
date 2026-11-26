package evalcensus

import "fmt"

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
