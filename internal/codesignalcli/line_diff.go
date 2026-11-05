package codesignalcli

// maxEditDistance bounds the shortest-edit-script search. A diff with more
// inserted and deleted lines than this is reported as a whole-file change,
// which over-marks lines as changed. A line whose content changed is never left
// unmarked; the lines are compared as content, so a change that only converts
// line endings is not seen when core.autocrlf is off (see withLineEndingsOf).
const maxEditDistance = 1000

// matchedNewLines runs Myers' shortest-edit-script search over two sequences of
// interned lines and reports, for each line of b, whether it belongs to a
// longest common subsequence with a. ok is false when the sequences differ by
// more than maxEditDistance edits.
func matchedNewLines(a, b []int) (matched []bool, ok bool) {
	if len(a) == 0 || len(b) == 0 {
		return make([]bool, len(b)), true
	}
	search := newEditSearch(a, b)
	for d := 0; d <= search.limit; d++ {
		if search.advance(d) {
			return search.matchedLines(), true
		}
	}
	return nil, false
}

// editSearch is the state of Myers' search: reach[offset+k] is the furthest x
// reached on diagonal k, and trace keeps that frontier as it stood before each
// edit so the winning path can be walked back.
type editSearch struct {
	a, b   []int
	limit  int
	offset int
	reach  []int
	trace  [][]int
}

func newEditSearch(a, b []int) editSearch {
	limit := min(len(a)+len(b), maxEditDistance)
	return editSearch{a: a, b: b, limit: limit, offset: limit + 1, reach: make([]int, 2*limit+3)}
}

// advance extends the frontier by the d-th edit and reports whether it now
// reaches the end of both sequences.
func (s *editSearch) advance(d int) bool {
	s.trace = append(s.trace, append([]int(nil), s.reach[s.offset-d-1:s.offset+d+2]...))
	for k := -d; k <= d; k += 2 {
		x := s.nextFurthest(d, k)
		x, y := s.followMatches(x, x-k)
		s.reach[s.offset+k] = x
		if x >= len(s.a) && y >= len(s.b) {
			return true
		}
	}
	return false
}

// nextFurthest is the x the search reaches on diagonal k after one more edit:
// down (an insertion) from diagonal k+1 or right (a deletion) from k-1,
// whichever got further.
func (s *editSearch) nextFurthest(d, k int) int {
	if k == -d || (k != d && s.reach[s.offset+k-1] < s.reach[s.offset+k+1]) {
		return s.reach[s.offset+k+1]
	}
	return s.reach[s.offset+k-1] + 1
}

// followMatches slides from (x, y) down the diagonal for as long as the two
// sequences hold the same line.
func (s *editSearch) followMatches(x, y int) (int, int) {
	for x < len(s.a) && y < len(s.b) && s.a[x] == s.b[y] {
		x++
		y++
	}
	return x, y
}

// matchedLines walks the recorded search back from the end and marks the
// lines of b that sit on diagonal runs, which are the matched ones.
func (s *editSearch) matchedLines() []bool {
	matched := make([]bool, len(s.b))
	x, y := len(s.a), len(s.b)
	for d := len(s.trace) - 1; d >= 0; d-- {
		snapshot := s.trace[d]
		k := x - y
		at := func(diagonal int) int { return snapshot[diagonal+d+1] }
		previousK := k - 1
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			previousK = k + 1
		}
		previousX := at(previousK)
		previousY := previousX - previousK
		for x > previousX && y > previousY {
			x--
			y--
			matched[y] = true
		}
		x, y = previousX, previousY
	}
	return matched
}
