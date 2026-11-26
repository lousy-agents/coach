package evalcensus

import "os"

// FailLedgerWritesAfter makes every ledger write stop after n bytes and
// fail, as a full disk or a killed process would, until restore is called.
func FailLedgerWritesAfter(n int) (restore func()) {
	saved := writeLedger
	writeLedger = func(file *os.File, data []byte) (int, error) {
		written, _ := file.Write(data[:min(n, len(data))])
		return written, os.ErrClosed
	}
	return func() { writeLedger = saved }
}

// OnRecordsSnapshot runs hook in AppendRun right after it notes the records'
// length, until restore is called, so a spec can race another writer there.
func OnRecordsSnapshot(hook func()) (restore func()) {
	saved := afterRecordsSnapshot
	afterRecordsSnapshot = hook
	return func() { afterRecordsSnapshot = saved }
}

// AppendRecords and AppendCorpusRecords are the unlocked-by-the-run appends
// that only AppendRun may call in production; specs drive them directly.
var (
	AppendRecords       = appendRecords
	AppendCorpusRecords = appendCorpusRecords
)

// FailFirstWriteTo makes the next write to the file at path stop after n
// bytes, run during while that write is still in progress, and then fail.
// Writes to any other file, and later writes, go through.
func FailFirstWriteTo(path string, n int, during func()) (restore func()) {
	saved := writeLedger
	failed := false
	writeLedger = func(file *os.File, data []byte) (int, error) {
		if failed || file.Name() != path {
			return saved(file, data)
		}
		failed = true
		written, _ := file.Write(data[:min(n, len(data))])
		during()
		return written, os.ErrClosed
	}
	return func() { writeLedger = saved }
}

// FindingKey is the part of a Coach finding that the census's sample orders by.
type FindingKey struct {
	RuleID, Path, Subject, Fingerprint string
	StartRow, EndRow                   uint
}

// CompareFindingKeys is the order a seeded draw sorts two findings by, so a spec can show that each key decides.
func CompareFindingKeys(a, b FindingKey) int {
	return compareFindings(a.signal(), b.signal())
}

func (k FindingKey) signal() reportSignal {
	signal := reportSignal{RuleID: k.RuleID, Path: k.Path, Subject: k.Subject, Fingerprint: k.Fingerprint}
	signal.Location.StartRow, signal.Location.EndRow = k.StartRow, k.EndRow
	return signal
}
