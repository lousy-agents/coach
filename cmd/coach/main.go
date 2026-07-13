// Command coach is the composition-root CLI for the coach project.
//
// Usage:
//
//	coach codesignal --base <ref> [--format text|json]
package main

import (
	"flag"
	"fmt"
	"os"
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

	_ = format
	fmt.Fprintln(stderr, "coach codesignal: not yet implemented")
	return 2
}
