package evalpatterns

import (
	"fmt"
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
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("file %s is not a regular file", rel)
	}
	return os.ReadFile(path)
}
