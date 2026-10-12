package sourcescope

import (
	"path/filepath"
	"regexp"
	"strings"
)

func globMatch(pattern, path string) bool {
	var expression strings.Builder
	expression.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			i = writeGlobStar(&expression, pattern, i)
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	expression.WriteString("$")
	return regexp.MustCompile(expression.String()).MatchString(filepath.ToSlash(path))
}

func writeGlobStar(expression *strings.Builder, pattern string, i int) int {
	if i+1 < len(pattern) && pattern[i+1] == '*' {
		i++
		if i+1 < len(pattern) && pattern[i+1] == '/' {
			i++
			expression.WriteString("(?:.*/)?")
			return i
		}
		expression.WriteString(".*")
		return i
	}
	expression.WriteString("[^/]*")
	return i
}
