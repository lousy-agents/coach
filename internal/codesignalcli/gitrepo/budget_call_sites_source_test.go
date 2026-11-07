package gitrepo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// subcommandsUnder is the subcommands asked in the Go files below root.
func subcommandsUnder(t *testing.T, root string) []string {
	t.Helper()
	var asked []string
	for _, path := range goSourceFilesUnder(t, root) {
		asked = append(asked, subcommandsAskedIn(t, path)...)
	}
	return asked
}

func subcommandsAsked(t *testing.T, roots ...string) []string {
	t.Helper()
	found := map[string]bool{}
	for _, root := range roots {
		for _, subcommand := range subcommandsUnder(t, root) {
			found[subcommand] = true
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// goSourceFilesUnder is the Go files below root that are not tests.
func goSourceFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr == nil && !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			paths = append(paths, path)
		}
		return walkErr
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return paths
}

func subcommandsAskedIn(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var asked []string
	ast.Inspect(file, func(node ast.Node) bool {
		asked = appendAsked(asked, node)
		return true
	})
	return asked
}
