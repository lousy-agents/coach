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

// ambiguousSlashWords are the words after which a slash could start a
// regular expression.
var ambiguousSlashWords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "case": true, "catch": true,
	"class": true, "const": true, "continue": true, "debugger": true, "default": true,
	"delete": true, "do": true, "else": true, "enum": true, "export": true, "extends": true,
	"finally": true, "for": true, "function": true, "if": true, "import": true, "in": true,
	"instanceof": true, "keyof": true, "let": true, "new": true, "of": true, "return": true,
	"satisfies": true, "static": true, "switch": true, "throw": true, "try": true,
	"typeof": true, "var": true, "void": true, "while": true, "with": true, "yield": true,
}

// slashDivides reports whether a slash after these tokens is certainly a
// division: it follows an identifier (including this, null and the other
// value words), a property name, a number, a string or a closing ]. Anywhere
// else it could start a regular expression, which is outside the subset the
// lexer reads.
func slashDivides(tokens []tsToken) bool {
	if len(tokens) == 0 {
		return false
	}
	prev, beforePrev := tokens[len(tokens)-1], tsToken{kind: tsOther}
	if len(tokens) > 1 {
		beforePrev = tokens[len(tokens)-2]
	}
	switch prev.kind {
	case tsIdent:
		return beforePrev.is(tsPunct, ".") || !ambiguousSlashWords[prev.text]
	case tsString, tsOther:
		return true
	case tsPunct:
		return prev.text == "]"
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
