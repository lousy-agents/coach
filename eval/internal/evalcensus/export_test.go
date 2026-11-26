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
