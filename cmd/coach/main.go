// Command coach is the composition-root CLI for the coach project.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// version identifies the coach binary. There is no build-time ldflags wiring
// yet; that is intentionally out of scope for this issue.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, topLevelUsage)
		return 2
	}

	switch args[0] {
	case "--help", "-h":
		fmt.Fprintln(stdout, topLevelUsage)
		return 0
	case "--version":
		fmt.Fprintln(stdout, version)
		return 0
	case "codesignal":
		return runCodesignal(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s\ncoach: unknown command %q\n", topLevelUsage, args[0])
		return 2
	}
}

const topLevelUsage = `usage: coach <command> [flags]

commands:
  codesignal   analyze production-code readiness signals in a Git diff or baseline

run "coach codesignal --help" for command-specific help.`

const codesignalUsage = "usage: coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--build-target <package>]"

func runCodesignal(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "git ref to diff against (mutually exclusive with --baseline)")
	baseline := flags.Bool("baseline", false, "scan every tracked file at HEAD instead of diffing against --base")
	format := flags.String("format", "text", "output format: text or json")
	scope := flags.String("scope", "production", "source scope: production or all")
	buildTarget := flags.String("build-target", "", "Go package pattern used to determine production reachability")

	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			var buffer bytes.Buffer
			flags.SetOutput(&buffer)
			flags.PrintDefaults()
			flags.SetOutput(stderr)
			fmt.Fprintln(stdout, codesignalUsage)
			fmt.Fprint(stdout, buffer.String())
			return 0
		}
	}

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *baseline && *base != "" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintln(stderr, "coach: --baseline and --base are mutually exclusive: choose a Repository Baseline scan (--baseline) or a diff comparison (--base), not both")
		return 2
	}

	if !*baseline && *base == "" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintln(stderr, "coach: missing required --base flag")
		return 2
	}

	if *format != "text" && *format != "json" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintf(stderr, "coach: invalid --format value %q: must be \"text\" or \"json\"\n", *format)
		return 2
	}
	if *scope != "production" && *scope != "all" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintf(stderr, "coach: invalid --scope value %q: must be \"production\" or \"all\"\n", *scope)
		return 2
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: cannot determine working directory: %s\n", err)
		return 1
	}

	var report *codesignal.Report
	if *baseline {
		revisionSHA, err := codesignalcli.ResolveBaselineRevision(dir)
		if err != nil {
			return reportOperationalError(err, stderr)
		}
		discovered, coverage, err := codesignalcli.DiscoverTrackedFiles(dir, revisionSHA)
		if err != nil {
			return reportOperationalError(err, stderr)
		}
		kept, excluded, err := codesignalcli.ApplyBaselineSourceScope(dir, revisionSHA, *buildTarget, *scope, discovered)
		if err != nil {
			return reportOperationalError(err, stderr)
		}
		coverage.Excluded = excluded
		report, err = codesignalcli.AnalyzeBaseline(context.Background(), dir, revisionSHA, kept, nil, coverage)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
			return 1
		}
	} else {
		headSHA, mergeBaseSHA, err := codesignalcli.ResolveRevisions(dir, *base)
		if err != nil {
			return reportOperationalError(err, stderr)
		}

		selected, diagnostics, err := codesignalcli.SelectChangedFiles(dir, mergeBaseSHA)
		if err != nil {
			return reportOperationalError(err, stderr)
		}
		selected, err = codesignalcli.ApplySourceScope(dir, headSHA, *buildTarget, *scope, selected)
		if err != nil {
			return reportOperationalError(err, stderr)
		}

		report, err = codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, selected, diagnostics)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
			return 1
		}
	}

	if *format == "json" {
		encoded, err := codesignalcli.RenderJSON(report)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: encoding report: %s\n", err)
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprint(stdout, codesignalcli.RenderText(report)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
		return 1
	}
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
