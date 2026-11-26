package evalhistory

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// Exit codes of `coach-eval history` (docs/evaluation/agentic-eval-design.md §10).
const (
	HistoryComplete       = 0
	HistoryInputInvalid   = 2
	HistoryAnalysisFailed = 3
)

// LedgerLine builds one history run's ledger line, newline included. The
// ledger's schema lives in the tooling ring, which this package may not
// import, so coach-eval supplies the builder.
type LedgerLine func(summary Summary, exit int, recorded time.Time) ([]byte, error)

// History runs `coach-eval history --coach <binary> --census-root <dir>
// --ledger <runs.jsonl> --seed <n> <corpus.json>`. It appends the run's
// association records and its ledger line as one unit, then prints the
// run's summary as JSON.
func History(args []string, stdout, stderr io.Writer, ledgerLine LedgerLine, now func() time.Time) int {
	summary, exit, err := history(args, ledgerLine)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval history: %v\n", err)
		return HistoryInputInvalid
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(stderr, "coach-eval history: %v\n", err)
		return HistoryInputInvalid
	}
	return exit
}

func history(args []string, ledgerLine LedgerLine) (Summary, int, error) {
	parsed, err := parseHistoryArgs(args)
	if err != nil {
		return Summary{}, 0, err
	}
	corpus, err := evalcensus.LoadCorpus(parsed.corpus)
	if err != nil {
		return Summary{}, 0, err
	}
	summary, err := Run(corpus, parsed.coach, parsed.seed)
	if err != nil {
		return Summary{}, 0, err
	}
	exit := HistoryComplete
	if summary.AnalysisFailed() {
		exit = HistoryAnalysisFailed
	}
	line, err := ledgerLine(summary, exit, time.Now().UTC())
	if err != nil {
		return Summary{}, 0, fmt.Errorf("build history ledger line: %w", err)
	}
	today := time.Now().UTC().Format(time.DateOnly)
	if err := evalcensus.AppendRun(parsed.ledger, line, parsed.censusRoot, summary.CorpusID, Records(summary, today)); err != nil {
		return Summary{}, 0, err
	}
	return summary, exit, nil
}
