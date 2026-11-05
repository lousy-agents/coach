package codesignalcli

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestChangedRangesBetween(t *testing.T) {
	tests := []struct {
		name       string
		base, head string
		want       []codesignal.LineRange
	}{
		{"identical", "a\nb\n", "a\nb\n", nil},
		{"appended lines", "a\nb\n", "a\nb\nc\nd\n", []codesignal.LineRange{{StartRow: 2, EndRow: 3}}},
		{"inserted in the middle", "a\nc\n", "a\nb\nc\n", []codesignal.LineRange{{StartRow: 1, EndRow: 1}}},
		{"a pure deletion changes no new line", "a\nb\nc\n", "a\nc\n", nil},
		{"a replaced line", "a\nb\nc\n", "a\nX\nc\n", []codesignal.LineRange{{StartRow: 1, EndRow: 1}}},
		{"two separate edits", "a\nb\nc\nd\ne\n", "a\nX\nc\nY\ne\n", []codesignal.LineRange{{StartRow: 1, EndRow: 1}, {StartRow: 3, EndRow: 3}}},
		{"an empty base marks every line", "", "a\nb\n", []codesignal.LineRange{{StartRow: 0, EndRow: 1}}},
		{"an empty head has no new lines", "a\nb\n", "", nil},
		{"a missing final newline changes the last line", "a\nb\n", "a\nb", []codesignal.LineRange{{StartRow: 1, EndRow: 1}}},
		{"a changed line ending is a changed line", "a\r\nb\r\n", "a\nb\r\n", []codesignal.LineRange{{StartRow: 0, EndRow: 0}}},
		{"CRLF in the head is not an edit when the base is stored with LF", "a\nb\nc\n", "a\r\nX\r\nc\r\n", []codesignal.LineRange{{StartRow: 1, EndRow: 1}}},
		{"a head that only gained CRLF endings changes no line", "a\nb\n", "a\r\nb\r\n", nil},
		{"a CRLF base keeps its endings, so an LF head differs on every line", "a\r\nb\r\n", "a\nb\n", []codesignal.LineRange{{StartRow: 0, EndRow: 1}}},
		{"a moved block is old lines plus a new copy", "a\nb\nc\nd\n", "c\nd\na\nb\n", []codesignal.LineRange{{StartRow: 2, EndRow: 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertChangedRanges(t, tt.base, tt.head, tt.want)
		})
	}
}

func assertChangedRanges(t *testing.T, base, head string, want []codesignal.LineRange) {
	t.Helper()

	got := changedRangesBetween([]byte(base), []byte(head))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("changedRangesBetween(%q, %q) = %+v, want %+v", base, head, got, want)
	}
}

func TestMatchedNewLinesFindsALongestCommonSubsequence(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	for round := 0; round < 400; round++ {
		a := randomLines(random, random.Intn(14))
		b := randomLines(random, random.Intn(14))

		assertMatchedLinesAreALongestCommonSubsequence(t, round, a, b)
	}
}

func assertMatchedLinesAreALongestCommonSubsequence(t *testing.T, round int, a, b []int) {
	t.Helper()

	matched, ok := matchedNewLines(a, b)

	if !ok {
		t.Fatalf("round %d: search gave up on a small input: a=%v b=%v", round, a, b)
	}
	common := matchedValues(matched, b)
	if !isSubsequence(common, a) {
		t.Fatalf("round %d: matched lines %v are not a subsequence of %v (b=%v)", round, common, a, b)
	}
	if want := longestCommonSubsequence(a, b); len(common) != want {
		t.Fatalf("round %d: matched %d lines, the longest common subsequence has %d (a=%v b=%v)", round, len(common), want, a, b)
	}
}

func TestChangedRangesBetweenMarksEveryLineWhenTheFilesDifferTooMuch(t *testing.T) {
	var base, head strings.Builder
	for i := 0; i < maxEditDistance+50; i++ {
		base.WriteString("old line\n")
		head.WriteString("new line\n")
	}

	got := changedRangesBetween([]byte(base.String()), []byte(head.String()))

	want := []codesignal.LineRange{{StartRow: 0, EndRow: uint(maxEditDistance + 49)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changedRangesBetween = %+v, want one range over every line %+v", got, want)
	}
}
