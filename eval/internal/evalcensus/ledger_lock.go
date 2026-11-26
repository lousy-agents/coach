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

// lockLedger takes the ledger's lock file, so concurrent writers check and
// append one at a time and a record_id is never written twice. A
// lock file works on every host the census runs on; one left by a crashed
// writer is reported by name once the wait runs out.
func lockLedger(path string) (func(), error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(ledgerLockWait)
	taken, err := tryLock(lockPath)
	for err == nil && !taken && time.Now().Before(deadline) {
		time.Sleep(ledgerLockPoll)
		taken, err = tryLock(lockPath)
	}
	switch {
	case err != nil:
		return nil, err
	case !taken:
		return nil, fmt.Errorf("ledger %s is locked by %s; remove it if no eval writer is running", path, lockPath)
	}
	return func() { os.Remove(lockPath) }, nil
}

// tryLock creates the lock file, reporting false when another writer holds it.
func tryLock(lockPath string) (bool, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, file.Close()
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
