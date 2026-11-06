package codesignalcli

import "testing"

func distinctLines(from, count int) []int {
	lines := make([]int, count)
	for i := range lines {
		lines[i] = from + i
	}
	return lines
}

func TestMatchedNewLinesAnswersAtTheEditDistanceBoundAndGivesUpBeyondIt(t *testing.T) {
	t.Run("exactly maxEditDistance edits", func(t *testing.T) {
		assertMatchedNewLines(t, distinctLines(0, maxEditDistance/2), distinctLines(10_000, maxEditDistance/2), true)
	})
	t.Run("one line past maxEditDistance edits", func(t *testing.T) {
		assertMatchedNewLines(t, distinctLines(0, maxEditDistance/2+1), distinctLines(10_000, maxEditDistance/2), false)
	})
}

func assertMatchedNewLines(t *testing.T, a, b []int, wantOK bool) {
	t.Helper()
	matched, ok := matchedNewLines(a, b)

	if ok != wantOK {
		t.Errorf("matchedNewLines ok = %v, want %v for %d old and %d new lines with nothing in common", ok, wantOK, len(a), len(b))
	}
	if !ok && matched != nil {
		t.Errorf("matchedNewLines gave up but returned matches %v", matched)
	}
}

func TestChangedRangesBetweenMarksEveryLineWhenOnlyOneLineSurvivesAHugeRewrite(t *testing.T) {
	var base, head []byte
	for i := 0; i < maxEditDistance; i++ {
		base = append(base, []byte("old "+string(rune('a'+i%26))+"\n")...)
		head = append(head, []byte("new "+string(rune('a'+i%26))+"\n")...)
		if i == maxEditDistance/2 {
			base = append(base, []byte("shared\n")...)
			head = append(head, []byte("shared\n")...)
		}
	}

	got := changedRangesBetween(base, head)

	if len(got) != 1 || got[0].StartRow != 0 || got[0].EndRow != uint(maxEditDistance) {
		t.Errorf("changedRangesBetween = %+v, want one range over every line, shared one included: the search gave up beyond maxEditDistance", got)
	}
}
