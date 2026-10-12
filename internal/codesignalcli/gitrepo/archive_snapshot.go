package gitrepo

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
)

func extractTar(dir string, archive []byte) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractTarEntry(dir, header, reader); err != nil {
			return err
		}
	}
}

func ExtractRevision(repositoryRoot, revision string) (string, error) {
	archive, err := RunBytes(repositoryRoot, "archive", "--format=tar", revision)
	if err != nil {
		return "", fmt.Errorf("reading source snapshot %q: %w", revision, err)
	}
	dir, err := os.MkdirTemp("", "coach-codesignal-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("creating source snapshot: %w", err)
	}
	if err := extractTar(dir, archive); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("extracting source snapshot: %w", err)
	}
	return dir, nil
}
