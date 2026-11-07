package codesignalcli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/lousy-agents/coach"

// productionFile is one non-test Go file of the module, parsed.
type productionFile struct {
	name   string // slash-separated, relative to the module root
	fset   *token.FileSet
	syntax *ast.File
}

// moduleProductionFiles parses the non-test Go files of every package of the module, for the current build.
func moduleProductionFiles(t *testing.T) []productionFile {
	t.Helper()
	root := moduleRoot(t)
	listing, err := exec.Command("go", "list", "-f", `{{.Dir}}{{range .GoFiles}}{{"\n"}}{{.}}{{end}}{{"\x00"}}`, modulePath+"/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var files []productionFile
	for _, listed := range strings.Split(string(listing), "\x00") {
		files = append(files, parseListedPackage(t, root, strings.Split(strings.TrimSpace(listed), "\n"))...)
	}
	return files
}

// parseListedPackage parses the files of one package of the go list output: its directory, then its file names.
func parseListedPackage(t *testing.T, root string, dirAndNames []string) []productionFile {
	t.Helper()
	if len(dirAndNames) < 2 {
		return nil
	}
	var files []productionFile
	for _, name := range dirAndNames[1:] {
		files = append(files, parseProductionFile(t, root, filepath.Join(dirAndNames[0], name)))
	}
	return files
}

func parseProductionFile(t *testing.T, root, full string) productionFile {
	t.Helper()
	fset := token.NewFileSet()
	syntax, err := parser.ParseFile(fset, full, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", full, err)
	}
	relative, err := filepath.Rel(root, full)
	if err != nil {
		t.Fatalf("relative path of %s: %v", full, err)
	}
	return productionFile{name: filepath.ToSlash(relative), fset: fset, syntax: syntax}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	output, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	return strings.TrimSpace(string(output))
}
