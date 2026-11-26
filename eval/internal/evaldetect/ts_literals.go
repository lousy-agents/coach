package evaldetect

import "strings"

// scanQuoted reads a quoted string, keeping the character after each
// backslash, which is enough to read module specifiers. A newline ends an
// unterminated string without being consumed, so the line count holds; a
// backslash-continued newline is counted as it is passed.
func (s *tsScanner) scanQuoted(quote byte) {
	line := s.line
	var text strings.Builder
	s.pos++
	for s.pos < len(s.src) && s.src[s.pos] != quote && s.src[s.pos] != '\n' {
		if s.src[s.pos] == '\\' && s.pos+1 < len(s.src) {
			s.pos++
		}
		text.WriteByte(s.src[s.pos])
		s.advanceTo(s.pos + 1)
	}
	if s.pos < len(s.src) && s.src[s.pos] == quote {
		s.pos++
	}
	s.emit(tsString, text.String(), line)
}

// scanTemplate reads template text up to its closing backtick or its next
// substitution. A head that reaches a substitution is emitted as a
// tsTemplate; a head that closes is a plain string. A tail, resumed after a
// substitution's closing brace, emits nothing more.
func (s *tsScanner) scanTemplate(line int, head bool) {
	var text strings.Builder
	for s.pos < len(s.src) {
		switch {
		case s.src[s.pos] == '\\' && s.pos+1 < len(s.src):
			text.WriteByte(s.src[s.pos+1])
			s.advanceTo(s.pos + 2)
		case s.src[s.pos] == '`':
			s.pos++
			if head {
				s.emit(tsString, text.String(), line)
			}
			return
		case strings.HasPrefix(s.src[s.pos:], "${"):
			s.pos += 2
			s.braces = append(s.braces, true)
			if head {
				s.emit(tsTemplate, text.String(), line)
			}
			return
		default:
			text.WriteByte(s.src[s.pos])
			s.advanceTo(s.pos + 1)
		}
	}
}
