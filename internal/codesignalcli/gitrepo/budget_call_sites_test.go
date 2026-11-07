package gitrepo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The table holds a row for every git subcommand the scan asks through RunBytes, Run or a Runner named run,
// found in the source of the scan and not in a list written beside the table. A subcommand without a row
// falls into the 30 s / 16 MiB default, which fails closed but is nobody's decision.

func subcommandsAsked(t *testing.T, roots ...string) []string {
	t.Helper()
	found := map[string]bool{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return walkErr
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if subcommand, asked := askedSubcommand(call); asked {
						found[subcommand] = true
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// askedSubcommand is the string literal that follows the directory in a call of RunBytes, Run or run.
func askedSubcommand(call *ast.CallExpr) (string, bool) {
	var name string
	switch function := call.Fun.(type) {
	case *ast.Ident:
		name = function.Name
	case *ast.SelectorExpr:
		name = function.Sel.Name
	}
	if (name != "RunBytes" && name != "Run" && name != "run") || len(call.Args) < 2 {
		return "", false
	}
	literal, ok := call.Args[1].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	subcommand, err := strconv.Unquote(literal.Value)
	return subcommand, err == nil
}

func TestEverySubcommandTheScanAsksHasARowInTheTable(t *testing.T) {
	asked := subcommandsAsked(t, "../../../internal", "../../../cmd")

	if len(asked) < 6 {
		t.Fatalf("found only %v: the scan of the source for calls is not finding them", asked)
	}
	unnamed := defaultBudgetFor([]string{"a-subcommand-no-one-named"})
	for _, subcommand := range asked {
		if defaultBudgetFor([]string{subcommand}) == unnamed {
			t.Errorf("git %s is asked by the scan and has no row in defaultBudgetFor: it gets the default %+v", subcommand, unnamed)
		}
	}
}

func TestAReadWithNoArgumentsGetsTheUnnamedBudgetAndDoesNotPanic(t *testing.T) {
	if got, want := defaultBudgetFor(nil), defaultBudgetFor([]string{"a-subcommand-no-one-named"}); got != want {
		t.Errorf("defaultBudgetFor(nil) = %+v, want the unnamed budget %+v", got, want)
	}
}
