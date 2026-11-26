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
// only after something that ends an expression. A keyword after a dot is a
// property name, which ends one.
func slashStartsRegex(tokens []tsToken) bool {
	if len(tokens) == 0 {
		return true
	}
	prev, beforePrev := tokens[len(tokens)-1], tsToken{kind: tsOther}
	if len(tokens) > 1 {
		beforePrev = tokens[len(tokens)-2]
	}
	switch prev.kind {
	case tsIdent:
		return regexKeywords[prev.text] && !beforePrev.is(tsPunct, ".")
	case tsPunct:
		return !endsOperand(prev, beforePrev)
	}
	return false
}

// endsOperand reports whether a punctuator closes an operand: a closing
// bracket, or the second sign of a postfix ++ or --.
func endsOperand(prev, beforePrev tsToken) bool {
	switch prev.text {
	case ")", "]", "}":
		return true
	case "+", "-":
		return beforePrev.is(tsPunct, prev.text)
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
