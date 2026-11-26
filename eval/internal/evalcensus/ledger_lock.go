package evalcensus

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

const (
	ledgerLockWait = 10 * time.Second
	ledgerLockPoll = 10 * time.Millisecond
)

// lockLedger takes the ledger's lock file, so concurrent writers load,
// check and append one at a time and a record_id is never written twice. A
// lock file works on every host the census runs on; one left by a crashed
// writer is reported by name once the wait runs out.
func lockLedger(path string) (func(), error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(ledgerLockWait)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			file.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("census ledger %s is locked by %s; remove it if no census is running", path, lockPath)
		}
		time.Sleep(ledgerLockPoll)
	}
}

// endsMidLine reports whether file's last byte is not a newline, as after a
// hand edit, so the next record would be glued onto that line.
func endsMidLine(file *os.File) bool {
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return false
	}
	last := make([]byte, 1)
	_, err = file.ReadAt(last, info.Size()-1)
	return err == nil && last[0] != '\n'
}
