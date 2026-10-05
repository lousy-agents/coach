package gitrepo

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractTarRegularFile(path string, header *tar.Header, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func extractTarSymlink(path string, header *tar.Header) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.Symlink(header.Linkname, path)
}

// safeTarEntryPath joins name onto dir and rejects any result that would
// escape dir (a path-traversal entry such as "../../etc/passwd").
func safeTarEntryPath(dir, name string) (string, error) {
	path := filepath.Join(dir, filepath.FromSlash(name))
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return path, nil
}

func extractTarEntry(dir string, header *tar.Header, reader *tar.Reader) error {
	path, err := safeTarEntryPath(dir, header.Name)
	if err != nil {
		return err
	}
	switch header.Typeflag {
	case tar.TypeXGlobalHeader, tar.TypeXHeader:
		// Metadata headers are consumed by archive/tar and do not represent
		// filesystem entries in the snapshot.
		return nil
	case tar.TypeDir:
		return os.MkdirAll(path, os.FileMode(header.Mode))
	case tar.TypeReg:
		return extractTarRegularFile(path, header, reader)
	case tar.TypeSymlink:
		return extractTarSymlink(path, header)
	default:
		return fmt.Errorf("unsupported archive entry %q", header.Name)
	}
}
