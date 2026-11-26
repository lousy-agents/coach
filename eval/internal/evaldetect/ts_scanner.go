package evaldetect

import "strings"

// tsScanner splits TS source into tokens, dropping comments and keeping the
// contents of strings and templates out of the token stream.
type tsScanner struct {
	src    string
	pos    int
	line   int
	tokens []tsToken
	// braces records, for each open brace, whether it opened a template
	// substitution, so the matching close brace resumes the template.
	braces []bool
	// refusedLine is the line of a slash the scanner could not classify, or 0.
	refusedLine int
}

// scanTS tokenizes src, stopping at the first slash it cannot classify as
// a division or a comment and returning that slash's line.
func scanTS(src string) ([]tsToken, int) {
	s := &tsScanner{src: src, line: 1}
	for s.pos < len(s.src) && s.refusedLine == 0 {
		s.step()
	}
	return s.tokens, s.refusedLine
}

func (s *tsScanner) step() {
	c := s.src[s.pos]
	switch {
	case c == '\n':
		s.line++
		s.pos++
	case c == ' ' || c == '\t' || c == '\r':
		s.pos++
	case strings.HasPrefix(s.src[s.pos:], "//"):
		s.skipLineComment()
	case strings.HasPrefix(s.src[s.pos:], "/*"):
		s.skipBlockComment()
	case c == '"' || c == '\'':
		s.scanQuoted(c)
	case c == '`':
		s.pos++
		s.scanTemplate(s.line, true)
	case isIdentByte(c):
		s.scanWord()
	default:
		s.scanPunct(c)
	}
}

func (s *tsScanner) emit(kind tsTokenKind, text string, line int) {
	s.tokens = append(s.tokens, tsToken{kind: kind, text: text, line: line})
}

func (s *tsScanner) skipLineComment() {
	for s.pos < len(s.src) && s.src[s.pos] != '\n' {
		s.pos++
	}
}

func (s *tsScanner) skipBlockComment() {
	end := strings.Index(s.src[s.pos+2:], "*/")
	if end < 0 {
		end = len(s.src) - s.pos - 2
	}
	s.advanceTo(s.pos + 2 + end + 2)
}

// advanceTo moves to offset, counting the newlines it passes.
func (s *tsScanner) advanceTo(offset int) {
	offset = min(offset, len(s.src))
	s.line += strings.Count(s.src[s.pos:offset], "\n")
	s.pos = offset
}

func (s *tsScanner) scanWord() {
	start := s.pos
	for s.pos < len(s.src) && isIdentByte(s.src[s.pos]) {
		s.pos++
	}
	kind := tsIdent
	if c := s.src[start]; c >= '0' && c <= '9' {
		kind = tsOther
	}
	s.emit(kind, s.src[start:s.pos], s.line)
}

func (s *tsScanner) scanPunct(c byte) {
	switch {
	case c == '/' && !slashDivides(s.tokens):
		s.refusedLine = s.line
		return
	case c == '{':
		s.braces = append(s.braces, false)
	case c == '}' && len(s.braces) > 0:
		opened := s.braces[len(s.braces)-1]
		s.braces = s.braces[:len(s.braces)-1]
		if opened {
			s.pos++
			s.scanTemplate(s.line, false)
			return
		}
	}
	s.emit(tsPunct, string(c), s.line)
	s.pos++
}
