package evalhistory

import (
	"errors"
	"flag"
	"io"
	"strconv"
)

// historyArgs is a parsed `coach-eval history` command line.
type historyArgs struct {
	coach, censusRoot, ledger, corpus string
	seed                              int64
}

func parseHistoryArgs(args []string) (historyArgs, error) {
	flags := flag.NewFlagSet("history", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var parsed historyArgs
	var seed string
	var online bool
	flags.StringVar(&parsed.coach, "coach", "", "the coach binary to scan with")
	flags.StringVar(&parsed.censusRoot, "census-root", "", "the directory holding each corpus's records")
	flags.StringVar(&parsed.ledger, "ledger", "", "the run ledger to append to")
	flags.StringVar(&seed, "seed", "", "the seed for the control draw")
	flags.BoolVar(&online, "online", false, "refused: the history runs offline only")
	if err := flags.Parse(args); err != nil {
		return historyArgs{}, err
	}
	if online {
		return historyArgs{}, errors.New("--online is refused: the history runs offline only, reading local clones")
	}
	parsed.corpus = flags.Arg(0)
	if err := historyArgsProblem(parsed, seed, flags.NArg()); err != nil {
		return historyArgs{}, err
	}
	parsed.seed, _ = strconv.ParseInt(seed, 10, 64)
	return parsed, nil
}

func historyArgsProblem(parsed historyArgs, seed string, positional int) error {
	if _, err := strconv.ParseInt(seed, 10, 64); err != nil {
		return errors.New("--seed <integer> is required, so the control draw can be repeated")
	}
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
