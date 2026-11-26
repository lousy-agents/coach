package antipatternfixtures

import (
	"fmt"
	"strings"
)

// Coach's thresholds for the complexity rules the templates trip, in the
// profile's units. Coach counts the function body as a block, so its
// nesting threshold of 4 is three nested constructs in the profile; its
// cognitive (15) and branch density (12) thresholds are reached at equal.
const (
	nestingThreshold   = 3
	cognitiveThreshold = 15
	densityThreshold   = 12
)

const arrowPath = "ledger/apply.go"

// arrowCode is #871's arrow code at the profile's nesting depth: two loops,
// then one condition per further level, with the work at the point. Its
// cognitive complexity is 1+2+…+depth, so from depth 5 it also trips
// cognitive complexity, and it declares that too. The fix keeps two
// constructs per function.
func arrowCode() Template {
	return Template{
		ID: "arrow-code", Name: "Arrow code (deep nesting)", Status: "covered",
		observe: func(p profile) (observation, bool) {
			return typical(p.Shapes.Nesting, "nesting depth", nestingThreshold, 8)
		},
		Bad:  arrowBad,
		Good: arrowGood,
		Expect: func(depth int) []Finding {
			findings := []Finding{{RuleID: "complexity.max_nesting_depth", Path: arrowPath}}
			if depth*(depth+1)/2 >= cognitiveThreshold {
				findings = append(findings, Finding{RuleID: "complexity.cognitive_complexity", Path: arrowPath, Subject: "Balance"})
			}
			return findings
		},
	}
}

// arrowCondition is the condition that opens nesting level n, from 3.
func arrowCondition(level int) string {
	switch level {
	case 3:
		return "value > 0"
	case 4:
		return "value <= limit"
	}
	return fmt.Sprintf("value != limit-%d", level-4)
}

func arrowBad(depth int) map[string]string {
	var body strings.Builder
	body.WriteString("\tfor _, batch := range batches {\n\t\tfor _, value := range batch {\n")
	for level := 3; level <= depth; level++ {
		fmt.Fprintf(&body, "%sif %s {\n", strings.Repeat("\t", level), arrowCondition(level))
	}
	fmt.Fprintf(&body, "%stotal += value\n", strings.Repeat("\t", depth+1))
	for level := depth; level >= 1; level-- {
		fmt.Fprintf(&body, "%s}\n", strings.Repeat("\t", level))
	}
	source := "package ledger\n\n" +
		fmt.Sprintf("// Balance sums the values that pass every check. Every check opens a deeper block, so the line that does the work\n// sits %d levels in and the reader must hold every enclosing condition in mind to know when it runs.\n", depth) +
		"func Balance(batches [][]int, limit int) int {\n\ttotal := 0\n" + body.String() + "\treturn total\n}\n"
	return map[string]string{"go.mod": goMod("ledger"), arrowPath: source}
}

func arrowGood(depth int) map[string]string {
	conditions := make([]string, 0, depth)
	for level := 3; level <= depth; level++ {
		conditions = append(conditions, arrowCondition(level))
	}
	source := "package ledger\n\n" +
		"// Balance sums the values that pass every check, one batch at a time; counts names the checks in one place.\n" +
		"func Balance(batches [][]int, limit int) int {\n\ttotal := 0\n" +
		"\tfor _, batch := range batches {\n\t\ttotal += batchBalance(batch, limit)\n\t}\n\treturn total\n}\n\n" +
		"func batchBalance(batch []int, limit int) int {\n\ttotal := 0\n" +
		"\tfor _, value := range batch {\n\t\tif counts(value, limit) {\n\t\t\ttotal += value\n\t\t}\n\t}\n" +
		"\treturn total\n}\n\n" +
		"func counts(value, limit int) bool {\n\treturn " + strings.Join(conditions, " && ") + "\n}\n"
	return map[string]string{"go.mod": goMod("ledger"), arrowPath: source}
}
