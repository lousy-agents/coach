package evalcensus

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Exit codes of `coach-eval census` (docs/evaluation/agentic-eval-design.md §10).
const (
	CensusComplete       = 0
	CensusInputInvalid   = 2
	CensusAnalysisFailed = 3
)

// LedgerLine builds one census run's ledger line, newline included. recorded
// is the run's one clock reading, which the records' date also comes from. The
// ledger's schema lives in the tooling ring, which this package may not
// import, so coach-eval supplies the builder.
type LedgerLine func(summary Summary, exit int, recorded time.Time) ([]byte, error)

// Census runs `coach-eval census --coach <binary> --census-root <dir>
// --ledger <runs.jsonl> <corpus.json>`. It appends each rule's status as a
// record under the census root and the run as a ledger line, then prints the
// run's summary as JSON.
func Census(args []string, stdout, stderr io.Writer, ledgerLine LedgerLine, now func() time.Time) int {
	parsed, err := parseCensusArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	summary, err := runCorpus(parsed.corpus, parsed.coach)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	exit := CensusComplete
	if summary.analysisFailed() {
		exit = CensusAnalysisFailed
	}
	if err := recordRun(parsed, summary, exit, ledgerLine, now().UTC()); err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	return exit
}

func recordRun(parsed censusArgs, summary Summary, exit int, ledgerLine LedgerLine, at time.Time) error {
	line, err := ledgerLine(summary, exit, at)
	if err != nil {
		return fmt.Errorf("build census ledger line: %w", err)
	}
	return AppendRun(parsed.ledger, line, parsed.censusRoot, summary.CorpusID, StatusRecords(summary, at.Format(time.DateOnly)))
}

func runCorpus(corpusPath, coach string) (Summary, error) {
	corpus, err := LoadCorpus(corpusPath)
	if err != nil {
		return Summary{}, err
	}
	if err := refuseTypeScript(corpus); err != nil {
		return Summary{}, err
	}
	return Run(corpus, coach)
}

// refuseTypeScript refuses a corpus with a TypeScript repository: Coach
// resolves a TypeScript compiler from the repository's own manifest and
// node_modules, which would run repository code on the host. #832 adds the
// pinned image that lifts this.
func refuseTypeScript(corpus Corpus) error {
	for _, repo := range corpus.Repositories {
		if repo.Language == "typescript" {
			return fmt.Errorf("repository %q is TypeScript, which the census scans only inside the pinned image (#832), because scanning it on the host would run its own compiler", repo.ID)
		}
	}
	return nil
}
