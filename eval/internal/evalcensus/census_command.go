package evalcensus

import (
	"encoding/json"
	"errors"
	"flag"
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

// LedgerWriter appends one census run's line to the ledger at path. The
// ledger's schema lives in the tooling ring, which this package may not
// import, so coach-eval supplies the writer.
type LedgerWriter func(path string, summary Summary, exit int) error

// censusArgs is a parsed `coach-eval census` command line.
type censusArgs struct {
	coach, censusRoot, ledger, corpus string
}

// Census runs `coach-eval census --coach <binary> --census-root <dir>
// --ledger <runs.jsonl> <corpus.json>`. It appends each rule's status as a
// record under the census root and the run as a ledger line, then prints the
// run's summary as JSON.
func Census(args []string, stdout, stderr io.Writer, writeLedger LedgerWriter) int {
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
	if err := recordRun(parsed, summary, exit, writeLedger); err != nil {
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

func recordRun(parsed censusArgs, summary Summary, exit int, writeLedger LedgerWriter) error {
	today := time.Now().UTC().Format(time.DateOnly)
	if err := AppendCorpusRecords(parsed.censusRoot, summary.CorpusID, StatusRecords(summary, today)); err != nil {
		return fmt.Errorf("write census records: %w", err)
	}
	if err := writeLedger(parsed.ledger, summary, exit); err != nil {
		return fmt.Errorf("write census ledger line: %w", err)
	}
	return nil
}

func parseCensusArgs(args []string) (censusArgs, error) {
	flags := flag.NewFlagSet("census", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var parsed censusArgs
	flags.StringVar(&parsed.coach, "coach", "", "the coach binary to scan with")
	flags.StringVar(&parsed.censusRoot, "census-root", "", "the directory holding each corpus's records")
	flags.StringVar(&parsed.ledger, "ledger", "", "the run ledger to append to")
	if err := flags.Parse(args); err != nil {
		return censusArgs{}, err
	}
	parsed.corpus = flags.Arg(0)
	return parsed, censusArgsProblem(parsed, flags.NArg())
}

func censusArgsProblem(parsed censusArgs, positional int) error {
	switch {
	case parsed.coach == "":
		return errors.New("--coach <coach binary> is required")
	case parsed.censusRoot == "":
		return errors.New("--census-root <directory> is required")
	case parsed.ledger == "":
		return errors.New("--ledger <runs.jsonl> is required")
	case positional != 1:
		return errors.New("expected exactly one corpus manifest")
	}
	return nil
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
