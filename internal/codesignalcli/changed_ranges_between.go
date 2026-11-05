package codesignalcli

import (
	"bytes"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// changedRangesBetween returns the line ranges of head that base does not
// contain, as `git diff --unified=0` reports them on the new side: runs of
// consecutive new lines, with a pure deletion contributing none. Both sides are
// bytes the scan already holds, so no git process reads the working tree, where
// a repository-configured clean filter could run or the file could have changed
// since it was read. A line's trailing newline is part of it, as it is for git.
func changedRangesBetween(base, head []byte) []codesignal.LineRange {
	baseLines, headLines := splitLines(base), splitLines(withLineEndingsOf(base, head))
	prefix := commonPrefix(baseLines, headLines)
	suffix := commonSuffix(baseLines[prefix:], headLines[prefix:])
	baseMiddle := baseLines[prefix : len(baseLines)-suffix]
	headMiddle := headLines[prefix : len(headLines)-suffix]

	interner := lineInterner{ids: map[string]int{}}
	matched, ok := matchedNewLines(interner.intern(baseMiddle), interner.intern(headMiddle))
	if !ok {
		matched = make([]bool, len(headMiddle))
	}
	return runsOfUnmatched(matched, prefix)
}

// withLineEndingsOf reads head with LF endings when base holds no CRLF. A
// repository that checks files out with CRLF (core.autocrlf=true, eol=crlf)
// stores them with LF, and git compares the stored form, so CRLF on disk is the
// checkout and not an edit. A base that holds CRLF is compared as it is.
func withLineEndingsOf(base, head []byte) []byte {
	if bytes.Contains(base, []byte("\r\n")) {
		return head
	}
	return bytes.ReplaceAll(head, []byte("\r\n"), []byte("\n"))
}

func splitLines(content []byte) [][]byte {
	lines := bytes.SplitAfter(content, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func commonPrefix(a, b [][]byte) int {
	n := 0
	for n < len(a) && n < len(b) && bytes.Equal(a[n], b[n]) {
		n++
	}
	return n
}

func commonSuffix(a, b [][]byte) int {
	n := 0
	for n < len(a) && n < len(b) && bytes.Equal(a[len(a)-1-n], b[len(b)-1-n]) {
		n++
	}
	return n
}

// lineInterner numbers each distinct line once, so lines interned through the
// same interner are equal exactly when their numbers are.
type lineInterner struct {
	ids map[string]int
}

func (in *lineInterner) intern(lines [][]byte) []int {
	interned := make([]int, len(lines))
	for i, line := range lines {
		id, seen := in.ids[string(line)]
		if !seen {
			id = len(in.ids)
			in.ids[string(line)] = id
		}
		interned[i] = id
	}
	return interned
}

// runsOfUnmatched groups consecutive unmatched lines into ranges, numbering
// rows from first, the count of unchanged lines before the compared middle.
func runsOfUnmatched(matched []bool, first int) []codesignal.LineRange {
	var ranges []codesignal.LineRange
	for start := 0; start < len(matched); {
		if matched[start] {
			start++
			continue
		}
		end := start
		for end < len(matched) && !matched[end] {
			end++
		}
		ranges = append(ranges, codesignal.LineRange{StartRow: uint(first + start), EndRow: uint(first + end - 1)})
		start = end
	}
	return ranges
}
