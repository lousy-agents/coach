package codesignalcli

import "math/rand"

// The helpers in this file are the independent oracle for matchedNewLines: a
// textbook longest-common-subsequence table and a subsequence check, written
// without any of the production algorithm, so the test does not recompute its
// expected value with the code it is testing.

func randomLines(random *rand.Rand, n int) []int {
	lines := make([]int, n)
	for i := range lines {
		lines[i] = random.Intn(4)
	}
	return lines
}

// matchedValues returns the values of b at the positions matched marks.
func matchedValues(matched []bool, b []int) []int {
	var common []int
	for j, isMatched := range matched {
		if isMatched {
			common = append(common, b[j])
		}
	}
	return common
}

func isSubsequence(sub, of []int) bool {
	next := 0
	for _, value := range of {
		if next < len(sub) && sub[next] == value {
			next++
		}
	}
	return next == len(sub)
}

func longestCommonSubsequence(a, b []int) int {
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			table[i][j] = longestCommonSubsequenceCell(table, i, j, a[i-1] == b[j-1])
		}
	}
	return table[len(a)][len(b)]
}

// longestCommonSubsequenceCell is the textbook recurrence for one table cell:
// a matching pair extends the diagonal, otherwise the better of the cell above
// and the cell to the left carries over.
func longestCommonSubsequenceCell(table [][]int, i, j int, pairMatches bool) int {
	if pairMatches {
		return table[i-1][j-1] + 1
	}
	return max(table[i-1][j], table[i][j-1])
}
