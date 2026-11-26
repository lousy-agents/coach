package evaldetect

import "fmt"

// tsImport is one literal module specifier and the line of the statement or
// call that names it.
type tsImport struct {
	specifier string
	line      int
}

type tsImportReader struct {
	file   string
	tokens []tsToken
}

func (r tsImportReader) at(i int) tsToken {
	if i < 0 || i >= len(r.tokens) {
		return tsToken{kind: tsOther}
	}
	return r.tokens[i]
}

// tsImports lists the static imports, re-exports, import() calls and
// require() calls in a TS source, refusing a call whose specifier is not a
// literal string.
func tsImports(file, src string) ([]tsImport, error) {
	r := tsImportReader{file: file, tokens: scanTS(src)}
	var imports []tsImport
	for i, token := range r.tokens {
		if token.kind != tsIdent || r.at(i-1).is(tsPunct, ".") {
			continue
		}
		found, ok, err := r.importAt(i, token)
		if err != nil {
			return nil, err
		}
		if ok {
			imports = append(imports, found)
		}
	}
	return imports, nil
}

func (r tsImportReader) importAt(i int, token tsToken) (tsImport, bool, error) {
	next := r.at(i + 1)
	switch token.text {
	case "import":
		return r.importForm(i, token, next)
	case "export":
		if next.is(tsPunct, "*") || next.is(tsPunct, "{") || (next.is(tsIdent, "type") && r.at(i+2).kind == tsPunct) {
			return r.fromClause(i, token)
		}
	case "require":
		if next.is(tsPunct, "(") && !r.at(i-1).is(tsIdent, "function") {
			return r.call(i, "require()")
		}
	}
	return tsImport{}, false, nil
}

func (r tsImportReader) importForm(i int, token, next tsToken) (tsImport, bool, error) {
	switch {
	case next.is(tsPunct, "("):
		return r.call(i, "import()")
	case next.is(tsPunct, "."):
		return tsImport{}, false, nil
	case next.kind == tsString:
		return tsImport{specifier: next.text, line: token.line}, true, nil
	}
	return r.fromClause(i, token)
}

// fromClause finds the `from "x"` that ends an import or export declaration,
// stopping at the end of the statement.
func (r tsImportReader) fromClause(i int, token tsToken) (tsImport, bool, error) {
	for j := i + 1; j < len(r.tokens); j++ {
		current := r.tokens[j]
		if current.is(tsPunct, ";") || current.is(tsPunct, "=") || current.is(tsIdent, "import") || current.is(tsIdent, "export") {
			break
		}
		if current.is(tsIdent, "from") && r.at(j+1).kind == tsString {
			return tsImport{specifier: r.at(j + 1).text, line: token.line}, true, nil
		}
	}
	return tsImport{}, false, nil
}

func (r tsImportReader) call(i int, name string) (tsImport, bool, error) {
	argument, after := r.at(i+2), r.at(i+3)
	if argument.kind == tsString && (after.is(tsPunct, ")") || after.is(tsPunct, ",")) {
		return tsImport{specifier: argument.text, line: r.tokens[i].line}, true, nil
	}
	return tsImport{}, false, fmt.Errorf("%w: %s:%d: a non-literal %s", ErrRefused, r.file, r.tokens[i].line, name)
}
