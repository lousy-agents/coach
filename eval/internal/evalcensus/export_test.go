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
