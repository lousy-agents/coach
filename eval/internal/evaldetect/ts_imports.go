package evaldetect

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
