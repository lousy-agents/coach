package projectmodel_test

import (
	"io/fs"
	"os"

	"path/filepath"
)

func body_tsSidecarIntegrationAcceptancePart3Test_20(p string, d fs.DirEntry, err error, src string, dst string) error {
	if err != nil {
		return err
	}
	rel, relErr := filepath.Rel(src, p)
	if relErr != nil {
		return relErr
	}
	target := filepath.Join(dst, rel)
	if d.IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	content, readErr := os.ReadFile(p)
	if readErr != nil {
		return readErr
	}
	return os.WriteFile(target, content, 0o644)
}
