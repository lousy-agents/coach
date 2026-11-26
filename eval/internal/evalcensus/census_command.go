package evalcensus

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
)

// Exit codes of `coach-eval census` (docs/evaluation/agentic-eval-design.md §10).
const (
	CensusComplete       = 0
	CensusInputInvalid   = 2
	CensusAnalysisFailed = 3
)

// Census runs `coach-eval census --coach <binary> <corpus.json>` and prints
// the run's summary as JSON.
func Census(args []string, stdout, stderr io.Writer) int {
	coach, corpusPath, err := parseCensusArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	summary, err := runCorpus(corpusPath, coach)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(stderr, "coach-eval census: %v\n", err)
		return CensusInputInvalid
	}
	if summary.analysisFailed() {
		return CensusAnalysisFailed
	}
	return CensusComplete
}

func parseCensusArgs(args []string) (string, string, error) {
	flags := flag.NewFlagSet("census", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	coach := flags.String("coach", "", "the coach binary to scan with")
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	switch {
	case *coach == "":
		return "", "", errors.New("--coach <coach binary> is required")
	case flags.NArg() != 1:
		return "", "", errors.New("expected exactly one corpus manifest")
	}
	return *coach, flags.Arg(0), nil
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
