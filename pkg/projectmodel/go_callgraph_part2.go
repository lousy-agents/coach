package projectmodel

import (
	"fmt"
	"go/token"

	"io/fs"
	"os"
	"path/filepath"
)

func relCallSitePath(tempDir string, pos token.Position) string {
	if pos.Filename == "" {
		return ""
	}
	rel, err := filepath.Rel(tempDir, pos.Filename)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.ToSlash(rel), pos.Line)
}

// materializeSnapshot copies snapshot into a new temporary directory so
// golang.org/x/tools/go/packages (which shells out to the Go toolchain) has
// real files to load; the caller must invoke the returned cleanup func.
func materializeSnapshot(snapshot fs.FS) (string, func(), error) {
	dir, err := os.MkdirTemp("", "projectmodel-callgraph-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := os.CopyFS(dir, snapshot); err != nil {
		cleanup()

		return dir, func() {}, err
	}
	return dir, cleanup, nil
}
