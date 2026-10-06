package sourcescope

import (
	"bytes"
)

// advanceInsideJSONString writes b (already known to be inside a JSON
// string literal) to out and returns the string/escape state after
// consuming it. Shared by stripJSONCComments and stripTrailingCommas so
// neither strips a comment- or comma-like byte that only appears inside a
// string value.
func advanceInsideJSONString(out *bytes.Buffer, b byte, escaped bool) (stillInString, stillEscaped bool) {
	out.WriteByte(b)
	switch {
	case escaped:
		return true, false
	case b == '\\':
		return true, true
	case b == '"':
		return false, false
	}
	return true, false
}

// skipJSONCLineComment returns the index of the '\n' terminating the "//"
// comment starting at data[i], or len(data) if it runs to EOF.
func skipJSONCLineComment(data []byte, i int) int {
	for i < len(data) && data[i] != '\n' {
		i++
	}
	return i
}

// stripJSONCComments strips // and /* */ outside strings, then trailing commas.
// Unterminated /* returns the original bytes so Unmarshal fails closed.
func stripJSONCComments(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			inString, escaped = advanceInsideJSONString(&out, b, escaped)
			continue
		}
		switch {
		case b == '"':
			inString = true
			out.WriteByte(b)
		case b == '/' && i+1 < len(data) && data[i+1] == '/':
			i = skipJSONCLineComment(data, i)
			if i < len(data) {
				out.WriteByte('\n')
			}
		case b == '/' && i+1 < len(data) && data[i+1] == '*':
			next, ok := skipJSONCBlockComment(data, i)
			if !ok {
				return data
			}
			i = next
		default:
			out.WriteByte(b)
		}
	}
	return stripTrailingCommas(out.Bytes())
}

// skipJSONCBlockComment returns the index of the '/' closing the "/* */"
// comment starting at data[i:i+2]. ok is false when the comment is
// unterminated.
func skipJSONCBlockComment(data []byte, i int) (newIndex int, ok bool) {
	i += 2
	for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
		i++
	}
	if i+1 >= len(data) {
		return 0, false
	}
	return i + 1, true
}
