package main

import (
	"flag"
	"fmt"

	"io"
	"os"
)

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
func parseCodesignalFlags(args []string, stdout, stderr *os.File) (codesignalFlags, int, bool) {
	suggestRequested := suggestProjectConfigRequested(args)

	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	if suggestRequested {
		flags.SetOutput(io.Discard)
	} else {
		flags.SetOutput(stderr)
	}
	holders := registerCodesignalFlags(flags)

	if handled, code := handleCodesignalHelp(args, flags, suggestRequested, stdout, stderr); handled {
		return codesignalFlags{}, code, false
	}

	if err := flags.Parse(args); err != nil {
		if suggestRequested {
			writeSuggestInvalidArguments(stderr, fmt.Sprintf("coach codesignal --suggest-project-config: invalid flags (project_config_suggestion_invalid_arguments): %s", err))
			return codesignalFlags{}, 2, false
		}
		return codesignalFlags{}, 2, false
	}

	parsed, exitCode, ok := finishCodesignalFlagParse(flags, holders, stderr)
	parsed.args = args
	return parsed, exitCode, ok
}
