package evalharness

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Exit codes of `coach-eval compare` and `coach-eval trend`, as design §10 names them.
const (
	CompareComparable    = 0
	CompareInputInvalid  = 2
	CompareNotComparable = 4
)

// Compare pairs the two runs named by args, writes the verdict in the requested format, and returns the
// exit code. Missing or invalid input is checked before any state or field comparison.
func Compare(stateRoot string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", formatText, "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return CompareInputInvalid
	}
	if !validOutputFormat(*format) {
		fmt.Fprintf(stderr, "coach-eval compare: --format must be text or json, not %q\n", *format)
		return CompareInputInvalid
	}
	runIDs := flags.Args()
	if slices.ContainsFunc(runIDs, namesFormatFlag) {
		fmt.Fprintln(stderr, "coach-eval compare: --format must come before the run ids")
		return CompareInputInvalid
	}
	if len(runIDs) != 2 {
		fmt.Fprintln(stderr, "coach-eval compare: expected exactly two run ids")
		return CompareInputInvalid
	}

	records := make([]map[string]any, len(runIDs))
	for i, runID := range runIDs {
		record, err := LoadRunRecord(stateRoot, runID)
		if err != nil {
			fmt.Fprintf(stderr, "coach-eval compare: %v\n", err)
			return CompareInputInvalid
		}
		records[i] = record
	}

	verdict := judgeRuns(runIDs, records)
	if *format == formatJSON {
		return writeCompareJSON(stdout, verdict)
	}
	return reportCompareText(stdout, stderr, verdict)
}

// namesFormatFlag reports whether a positional argument is the format flag in any of its spellings. The
// flag package stops at the first positional argument, so a --format written after the run ids arrives
// here as one.
func namesFormatFlag(arg string) bool {
	name, _, _ := strings.Cut(arg, "=")
	return name == "--format" || name == "-format"
}

func reportCompareText(stdout, stderr io.Writer, verdict compareVerdict) int {
	if len(verdict.notComplete) > 0 {
		fmt.Fprintf(stderr, "coach-eval compare: not comparable; only complete runs compare: %s\n", joinNotComplete(verdict.notComplete))
		return CompareNotComparable
	}
	if len(verdict.differing) > 0 {
		fmt.Fprintf(stderr, "coach-eval compare: not comparable; these fields differ: %s\n", strings.Join(verdict.differing, ", "))
		return CompareNotComparable
	}
	for _, warning := range verdict.warnings {
		fmt.Fprintf(stderr, "coach-eval compare: warning: %s\n", warning)
	}
	fmt.Fprintf(stdout, "comparable: %s and %s\n", verdict.runIDs[0], verdict.runIDs[1])
	return CompareComparable
}
