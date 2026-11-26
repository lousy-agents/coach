package evaldetect

type tsTokenKind int

const (
	tsIdent tsTokenKind = iota
	tsString
	tsTemplate
	tsPunct
	tsOther
)

// tsToken is one significant token of TS source. A tsString is a quoted
// string or a template with no substitution; a tsTemplate has a substitution
// and so is never a literal specifier.
type tsToken struct {
	kind tsTokenKind
	text string
	line int
}

func (t tsToken) is(kind tsTokenKind, text string) bool {
	return t.kind == kind && t.text == text
}

// regexKeywords are the identifiers after which a slash starts a regular
// expression rather than a division.
var regexKeywords = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true,
	"delete": true, "void": true, "throw": true, "case": true, "do": true, "else": true,
	"yield": true, "await": true,
}

// slashStartsRegex applies the usual lexer heuristic: a slash is a division
// only after something that ends an expression.
func slashStartsRegex(tokens []tsToken) bool {
	if len(tokens) == 0 {
		return true
	}
	prev := tokens[len(tokens)-1]
	switch prev.kind {
	case tsIdent:
		return regexKeywords[prev.text]
	case tsPunct:
		return prev.text != ")" && prev.text != "]" && prev.text != "}"
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
