package evalpatterns

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// readRegularFile reads the checkout file at the slash-separated rel. A file
// that is not regular, or a directory on the way to it that is not a
// directory, such as a symlink, is an error rather than a path to follow:
// the checkout is untrusted, and a link could reach a file on the host.
func readRegularFile(root, rel string) ([]byte, error) {
	if err := requireDirectories(root, rel); err != nil {
		return nil, err
	}
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

// requireDirectories checks that each directory below root on the way to
// rel is a real directory. A missing one is fs.ErrNotExist.
func requireDirectories(root, rel string) error {
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return withoutHostPath(rel, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("file %s cannot be read: %s is %s", rel, dir, notADirectory(info.Mode()))
		}
	}
	return nil
}

func notADirectory(mode fs.FileMode) string {
	if mode.IsRegular() {
		return "not a directory"
	}
	return "not a regular file or directory"
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

// checkoutError is err with its path, which a walk of root gives under
// root, made relative to the checkout.
func checkoutError(root string, err error) error {
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		return err
	}
	rel, _ := filepath.Rel(root, pathErr.Path)
	return withoutHostPath(filepath.ToSlash(rel), err)
}
