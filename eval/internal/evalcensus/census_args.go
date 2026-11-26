package evalcensus

import (
	"errors"
	"flag"
	"io"
)

// censusArgs is a parsed `coach-eval census` command line.
type censusArgs struct {
	coach, censusRoot, ledger, corpus string
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
