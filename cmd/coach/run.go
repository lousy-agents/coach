package main

import (
	"fmt"
	"os"
)

const topLevelUsage = `usage: coach <command> [flags]

commands:
  codesignal   analyze production-code readiness signals in a Git diff or baseline

run "coach codesignal --help" for command-specific help.`

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

func runCodesignal(args []string, stdout, stderr *os.File) int {
	parsed, exitCode, ok := parseCodesignalFlags(args, stdout, stderr)
	if !ok {
		return exitCode
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: cannot determine working directory: %s\n", err)
		return 1
	}

	if parsed.suggestProjectConfig {
		if parsed.projectLanguage == "typescript" {
			return runAuthorProjectConfigTypeScript(dir, parsed, stdout, stderr)
		}
		return runSuggestProjectConfig(dir, parsed, stdout, stderr)
	}

	if parsed.prepareCompiler {
		return runPrepareCompilerMiseTypeScript(dir, parsed, os.Stdin, stdout, stderr)
	}

	if parsed.checkProject {
		return runCheckProject(dir, parsed, stdout, stderr)
	}

	return runCodesignalScan(dir, parsed, stdout, stderr, newScanOfferBudget())
}
