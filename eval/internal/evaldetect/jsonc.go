package evaldetect

import "strings"

// stripJSONC removes the comments and trailing commas tsconfig files allow,
// leaving strings intact, so encoding/json can read the result.
func stripJSONC(src string) string {
	var out strings.Builder
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '"':
			end := jsonStringEnd(src, i)
			out.WriteString(src[i:end])
			i = end - 1
		case strings.HasPrefix(src[i:], "//"):
			i += strings.IndexByte(src[i:]+"\n", '\n') - 1
		case strings.HasPrefix(src[i:], "/*"):
			i += strings.Index(src[i:]+"*/", "*/") + 1
		case src[i] == ',' && closesNext(src[i+1:]):
		default:
			out.WriteByte(src[i])
		}
	}
	return out.String()
}

func jsonStringEnd(src string, start int) int {
	for i := start + 1; i < len(src); i++ {
		if src[i] == '\\' {
			i++
		} else if src[i] == '"' {
			return i + 1
		}
	}
	return len(src)
}

// closesNext reports whether the next significant character closes an object
// or array, which makes a preceding comma a trailing one.
func closesNext(rest string) bool {
	trimmed := strings.TrimLeft(stripLeadingComments(rest), " \t\r\n")
	return strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "]")
}

func stripLeadingComments(rest string) string {
	for {
		trimmed := strings.TrimLeft(rest, " \t\r\n")
		switch {
		case strings.HasPrefix(trimmed, "//"):
			rest = trimmed[strings.IndexByte(trimmed+"\n", '\n'):]
		case strings.HasPrefix(trimmed, "/*"):
			rest = trimmed[strings.Index(trimmed+"*/", "*/")+2:]
		default:
			return trimmed
		}
	}
}
