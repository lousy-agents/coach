package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

type withheldClause struct {
	flag   string
	phrase string
	count  int
}

func withheldClauses(withheld *codesignal.SignalsWithheld) []withheldClause {
	var clauses []withheldClause
	if withheld.MinSeverity != "" {
		clauses = append(clauses, withheldClause{"--min-severity", "below --min-severity " + string(withheld.MinSeverity), withheld.BelowMinSeverity})
	}
	if withheld.Top > 0 {
		clauses = append(clauses, withheldClause{"--top", fmt.Sprintf("beyond --top %d", withheld.Top), withheld.BeyondTop})
	}
	return clauses
}

func renderWithheldSignals(b *strings.Builder, withheld *codesignal.SignalsWithheld, seeAllCommand string) {
	if withheld == nil {
		return
	}
	clauses := withheldClauses(withheld)
	if len(clauses) == 0 {
		return
	}

	total := 0
	for _, clause := range clauses {
		total += clause.count
	}
	fmt.Fprintf(b, "withheld: %s %s", signalCountNoun(total), withheldBreakdown(clauses))
	b.WriteString("; summary counts describe the full analysis")
	if total > 0 {
		if seeAllCommand == "" {
			seeAllCommand = seeAllWithoutFlags(clauses)
		}
		fmt.Fprintf(b, "; see all: %s", seeAllCommand)
	}
	b.WriteString("\n")
}

func seeAllWithoutFlags(clauses []withheldClause) string {
	flags := make([]string, len(clauses))
	for i, clause := range clauses {
		flags[i] = clause.flag
	}
	return "re-run without " + strings.Join(flags, " and ")
}

func withheldBreakdown(clauses []withheldClause) string {
	if len(clauses) == 1 {
		return clauses[0].phrase
	}
	return fmt.Sprintf("(%d %s, %d %s)", clauses[0].count, clauses[0].phrase, clauses[1].count, clauses[1].phrase)
}

func signalCountNoun(n int) string {
	if n == 1 {
		return "1 signal"
	}
	return fmt.Sprintf("%d signals", n)
}
