package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

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

func finishCodesignalFlagParse(flags *flag.FlagSet, h codesignalFlagHolders, stderr *os.File) (codesignalFlags, int, bool) {
	setFlags := map[string]bool{}
	flags.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	parsed := codesignalFlagsFromHolders(h, setFlags)

	if parsed.suggestProjectConfig {
		if errMsg := validateSuggestProjectConfigFlags(parsed, setFlags, flags.Args(), h.suggestProjectConfig.count, h.output.count); errMsg != "" {
			writeSuggestInvalidArguments(stderr, errMsg)
			return codesignalFlags{}, 2, false
		}
		return parsed, 0, true
	}

	if parsed.prepareCompiler {
		if errMsg := validatePrepareCompilerFlags(parsed, setFlags, flags.Args(), h.prepareCompiler.count); errMsg != "" {
			fmt.Fprintln(stderr, codesignalUsage)
			fmt.Fprintln(stderr, errMsg)
			return codesignalFlags{}, 2, false
		}
		return parsed, 0, true
	}

	if parsed.checkProject {
		if errMsg := validateCheckProjectFlags(parsed, setFlags, flags.Args(), h.checkProject.count); errMsg != "" {
			fmt.Fprintln(stderr, codesignalUsage)
			fmt.Fprintln(stderr, errMsg)
			return codesignalFlags{}, 2, false
		}
		return parsed, 0, true
	}

	if errMsg := validateCodesignalFlags(parsed, flags.Args()); errMsg != "" {
		fmt.Fprintln(stderr, codesignalUsage)
		fmt.Fprintln(stderr, errMsg)
		return codesignalFlags{}, 2, false
	}
	return parsed, 0, true
}
