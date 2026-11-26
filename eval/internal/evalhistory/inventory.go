package evalhistory

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// Inventory is the production Go code at a SHA: every function, and every
// file as a whole.
type Inventory struct {
	Functions []Unit
	Files     []Unit
}

// GoInventory parses each Go file at since that repo's census scope admits,
// read from git without a checkout. The scope is the census's own, so the
// history's controls and flagged units come from the code the census counts.
func GoInventory(repoDir, since string, repo evalcensus.Repository) (Inventory, error) {
	listing, err := git(repoDir, "ls-tree", "-r", "-z", "--name-only", since)
	if err != nil {
		return Inventory{}, err
	}
	var inventory Inventory
	for _, path := range strings.Split(listing, "\x00") {
		if !strings.HasSuffix(path, ".go") || !repo.Admits(path) {
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
