package sourcescope

import (
	"bytes"
)

// trailingCommaFollowedByClose reports whether the comma at data[i] is
// followed only by whitespace before a closing '}' or ']', making it a
// JSONC trailing comma to drop rather than emit.
func trailingCommaFollowedByClose(data []byte, i int) bool {
	j := i + 1
	for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
		j++
	}
	return j < len(data) && (data[j] == '}' || data[j] == ']')
}

func stripTrailingCommas(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			inString, escaped = advanceInsideJSONString(&out, b, escaped)
			continue
		}
		if b == '"' {
			inString = true
			out.WriteByte(b)
			continue
		}
		if b == ',' && trailingCommaFollowedByClose(data, i) {
			continue
		}
		out.WriteByte(b)
	}
	return out.Bytes()
}
