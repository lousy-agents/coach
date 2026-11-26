package evalpatterns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// readRegularFile reads the checkout file at the slash-separated rel. A file
// that is not regular, such as a symlink, is an error rather than a file to
// read: the checkout is untrusted, and a link could reach a file on the host.
func readRegularFile(root, rel string) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, withoutHostPath(rel, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("file %s is not a regular file", rel)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, withoutHostPath(rel, err)
	}
	return raw, nil
}

// withoutHostPath names rel, not the host path the checkout sits under,
// and keeps the cause so a missing file is still fs.ErrNotExist.
func withoutHostPath(rel string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return fmt.Errorf("file %s cannot be read: %w", rel, err)
}
