package evalhistory

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// Inventory is the production Go code at a SHA: every function, and every
// file as a whole.
type Inventory struct {
	Functions []Unit
	Files     []Unit
}

// GoInventory parses each production Go file at since, read from git
// without a checkout. Test files and vendor, testdata and hidden directories
// are left out, as Coach's production scan leaves them out.
func GoInventory(repoDir, since string) (Inventory, error) {
	listing, err := git(repoDir, "ls-tree", "-r", "-z", "--name-only", since)
	if err != nil {
		return Inventory{}, err
	}
	var inventory Inventory
	for _, path := range strings.Split(listing, "\x00") {
		if !isProductionGo(path) {
			continue
		}
		if err := inventory.add(repoDir, since, path); err != nil {
			return Inventory{}, err
		}
	}
	sortUnits(inventory.Functions)
	sortUnits(inventory.Files)
	return inventory, nil
}

func (inventory *Inventory) add(repoDir, since, path string) error {
	source, err := git(repoDir, "show", since+":"+path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return fmt.Errorf("parse %s at %s: %w", path, since, err)
	}
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok {
			inventory.Functions = append(inventory.Functions, Unit{Path: path, Start: fset.Position(function.Pos()).Line, End: fset.Position(function.End()).Line})
		}
	}
	inventory.Files = append(inventory.Files, Unit{Path: path, Start: 1, End: lineCount(source)})
	return nil
}

func isProductionGo(path string) bool {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "vendor" || segment == "testdata" || strings.HasPrefix(segment, ".") {
			return false
		}
	}
	return true
}

func lineCount(source string) int {
	return strings.Count(strings.TrimSuffix(source, "\n"), "\n") + 1
}

func sortUnits(units []Unit) {
	sort.Slice(units, func(i, j int) bool {
		if units[i].Path != units[j].Path {
			return units[i].Path < units[j].Path
		}
		return units[i].Start < units[j].Start
	})
}
