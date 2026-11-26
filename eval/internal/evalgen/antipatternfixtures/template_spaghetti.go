package antipatternfixtures

import (
	"fmt"
	"strings"
)

const (
	spaghettiPath  = "archive/select.go"
	spaghettiRules = 4
)

// spaghettiConditional is #871's spaghetti conditional at the profile's
// cognitive complexity: one loop (1) holding four rules (2 each, nested
// once), whose conditions mix && and || so that every change of operator
// adds one more (Coach counts one per run of identical operators, as
// gocognit does). Few branches keep it under the god-file threshold.
func spaghettiConditional() Template {
	return Template{
		ID: "spaghetti-conditional", Name: "Spaghetti conditional (high cognitive load)", Status: "covered",
		observe: func(p profile) (observation, bool) {
			return typical(p.Shapes.Cognitive, "cognitive complexity", cognitiveThreshold, 40)
		},
		Bad:  spaghettiBad,
		Good: spaghettiGood,
		Expect: func(int) []Finding {
			return []Finding{{RuleID: "complexity.cognitive_complexity", Path: spaghettiPath, Subject: "Expired"}}
		},
	}
}

// spaghettiConditions are the four rules' conditions, whose operator runs
// add up to cognitive-1-2*4, so the function scores exactly cognitive.
func spaghettiConditions(cognitive int) []string {
	runs := cognitive - 1 - 2*spaghettiRules
	conditions := make([]string, spaghettiRules)
	flag := 0
	for rule := range conditions {
		count := runs / spaghettiRules
		if rule < runs%spaghettiRules {
			count++
		}
		conditions[rule], flag = mixedCondition(flag, count)
	}
	return conditions
}

// mixedCondition chains runs+1 flags from first, alternating && and ||, so
// it holds exactly runs operator runs. It returns the next unused flag.
func mixedCondition(first, runs int) (string, int) {
	terms := []string{fmt.Sprintf("f.Flags[%d]", first)}
	for run := 1; run <= runs; run++ {
		operator := " && "
		if run%2 == 0 {
			operator = " || "
		}
		terms = append(terms, operator, fmt.Sprintf("f.Flags[%d]", first+run))
	}
	return strings.Join(terms, ""), first + runs + 1
}

const spaghettiFile = "package archive\n\n// File is a file in a directory listing, with the flags the deletion rules read.\ntype File struct {\n\tName  string\n\tFlags []bool\n}\n\n"

func spaghettiBad(cognitive int) map[string]string {
	var body strings.Builder
	for _, condition := range spaghettiConditions(cognitive) {
		fmt.Fprintf(&body, "\t\tif %s {\n\t\t\tnames = append(names, f.Name)\n\t\t\tcontinue\n\t\t}\n", condition)
	}
	source := spaghettiFile +
		"// Expired picks the files to delete. Each rule mixes && and || in one condition, so no rule can be read, tested or\n// changed on its own.\n" +
		"func Expired(files []File) []string {\n\tvar names []string\n\tfor _, f := range files {\n" + body.String() + "\t}\n\treturn names\n}\n"
	return map[string]string{"go.mod": goMod("files"), spaghettiPath: source}
}

func spaghettiGood(cognitive int) map[string]string {
	conditions := spaghettiConditions(cognitive)
	calls := make([]string, len(conditions))
	var rules strings.Builder
	for i, condition := range conditions {
		calls[i] = fmt.Sprintf("rule%d(f)", i+1)
		fmt.Fprintf(&rules, "\nfunc rule%d(f File) bool {\n\treturn %s\n}\n", i+1, condition)
	}
	source := spaghettiFile +
		"// Expired picks the files that any one deletion rule selects; each rule is a function of its own.\n" +
		"func Expired(files []File) []string {\n\tvar names []string\n\tfor _, f := range files {\n" +
		"\t\tif " + strings.Join(calls, " || ") + " {\n\t\t\tnames = append(names, f.Name)\n\t\t}\n\t}\n\treturn names\n}\n" + rules.String()
	return map[string]string{"go.mod": goMod("files"), spaghettiPath: source}
}
