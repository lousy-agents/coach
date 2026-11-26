package evaldetect

import "fmt"

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
