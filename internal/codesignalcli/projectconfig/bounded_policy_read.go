package projectconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func readBounded(root *os.Root, repoPath string) ([]byte, error) {
	file, err := root.OpenFile(filepath.FromSlash(repoPath), os.O_RDONLY|openWithoutWaiting, 0)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %s", repoPath, causeText(err))
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("reading %s: not a regular file", repoPath)
	}
	content, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %s", repoPath, causeText(err))
	}
	if int64(len(content)) > MaxBytes {
		return nil, &oversizePolicyError{path: repoPath}
	}
	return content, nil
}

// oversizePolicyError is typed so a caller can tell a policy over the size limit from one it could not read.
type oversizePolicyError struct{ path string }

func (err *oversizePolicyError) Error() string {
	return fmt.Sprintf("%s exceeds the %d-byte size budget", err.path, MaxBytes)
}
