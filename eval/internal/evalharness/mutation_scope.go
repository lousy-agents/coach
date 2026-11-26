package evalharness

import (
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// MutationScope lists the directories of the packages reachable in the program's import graph from the api surface
// (design §6, step 6): the packages of the surface's symbols and every package of the module they import, directly or
// through others. A test file's imports do not extend it, and a file whose imports cannot be read is an error rather
// than a smaller scope.
func MutationScope(program evalgen.Program) ([]string, error) {
	imports, err := moduleImports(program.Tree, moduleNameOf(program))
	if err != nil {
		return nil, err
	}
	reached := map[string]bool{}
	pending := routeDirectories(program)
	for len(pending) > 0 {
		directory := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if reached[directory] {
			continue
		}
		reached[directory] = true
		pending = append(pending, imports[directory]...)
	}
	scope := make([]string, 0, len(reached))
	for directory := range reached {
		scope = append(scope, directory)
	}
	sort.Strings(scope)
	return scope, nil
}

// moduleImports maps each directory of non-test Go files to the module's directories its files import.
func moduleImports(tree map[string][]byte, module string) (map[string][]string, error) {
	imports := map[string][]string{}
	files := token.NewFileSet()
	for name, source := range tree {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(files, name, source, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, err
			}
			if internal, ok := strings.CutPrefix(imported, module+"/"); ok {
				imports[path.Dir(name)] = append(imports[path.Dir(name)], internal)
			}
		}
	}
	return imports, nil
}

func moduleNameOf(program evalgen.Program) string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(program.Tree["go.mod"]), "module "), "\n")
	return first
}
