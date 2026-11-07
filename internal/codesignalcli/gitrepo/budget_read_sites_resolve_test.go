package gitrepo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// What a call of a bounded read passes as its limits, found in the source: the files of a package.

func parseFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

// nonTestFiles is the Go files of the package in dir that are not tests.
func nonTestFiles(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
	}
	return files
}
