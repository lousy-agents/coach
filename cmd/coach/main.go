// Command coach is the composition-root CLI for the coach project.
//
// Usage:
//
//	coach codesignal --base <ref> [--format text|json]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: coach codesignal --base <ref> [--format text|json]")
		return 2
	}

	switch args[0] {
	case "codesignal":
		return runCodesignal(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "usage: coach codesignal --base <ref> [--format text|json]\ncoach: unknown command %q\n", args[0])
		return 2
	}
}

func runCodesignal(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "git ref to diff against (required)")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *base == "" {
		fmt.Fprintln(stderr, "usage: coach codesignal --base <ref> [--format text|json]")
		fmt.Fprintln(stderr, "coach: missing required --base flag")
		return 2
	}

	if *format != "text" && *format != "json" {
		fmt.Fprintln(stderr, "usage: coach codesignal --base <ref> [--format text|json]")
		fmt.Fprintf(stderr, "coach: invalid --format value %q: must be \"text\" or \"json\"\n", *format)
		return 2
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: cannot determine working directory: %s\n", err)
		return 1
	}

	headSHA, mergeBaseSHA, err := codesignalcli.ResolveRevisions(dir, *base)
	if err != nil {
		return reportOperationalError(err, stderr)
	}

	selected, diagnostics, err := codesignalcli.SelectChangedFiles(dir, mergeBaseSHA)
	if err != nil {
		return reportOperationalError(err, stderr)
	}

	report, err := codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, selected, diagnostics)
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
		return 1
	}

	if *format == "json" {
		encoded, err := codesignalcli.RenderJSON(report)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: encoding report: %s\n", err)
			return 1
		}
		stdout.Write(encoded)
		return 0
	}

	fmt.Fprint(stdout, codesignalcli.RenderText(report))
	return 0
}

func reportOperationalError(err error, stderr *os.File) int {
	var opErr *codesignalcli.OperationalError
	if errors.As(err, &opErr) {
		fmt.Fprintln(stderr, opErr.Message)
		return 1
	}
	fmt.Fprintln(stderr, err)
	return 1
}
