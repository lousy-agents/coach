package rubrics

import (
	"fmt"
	"strings"
)

// FormatSpanWindow returns a numbered excerpt of content centered on startRow
// (0-based line index). radius is the inclusive line count above and below;
// when radius is zero, DefaultEvidenceWindowLines (15) is used. Lines outside
// the file are omitted. Each line is formatted as "  N|text" (1-based display
// numbers) with a leading ">" marker on the startRow line.
func FormatSpanWindow(content string, startRow, radius int) string {
	if radius <= 0 {
		radius = DefaultEvidenceWindowLines
	}
	// Split preserving trailing empty only when content ends with newline-less last line.
	lines := splitContentLines(content)
	if len(lines) == 0 {
		return ""
	}
	if startRow < 0 {
		startRow = 0
	}
	if startRow >= len(lines) {
		startRow = len(lines) - 1
	}
	lo := startRow - radius
	if lo < 0 {
		lo = 0
	}
	hi := startRow + radius
	if hi >= len(lines) {
		hi = len(lines) - 1
	}

	width := len(fmt.Sprintf("%d", hi+1))
	var b strings.Builder
	for i := lo; i <= hi; i++ {
		marker := " "
		if i == startRow {
			marker = ">"
		}
		// 1-based display line number.
		fmt.Fprintf(&b, "%s %*d|%s\n", marker, width, i+1, lines[i])
	}
	return b.String()
}

func splitContentLines(content string) []string {
	if content == "" {
		return nil
	}
	// strings.Split keeps a trailing empty element when content ends with \n;
	// drop that so line counts match editor/display conventions.
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
