package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

func handleCodesignalHelp(args []string, flags *flag.FlagSet, suggestRequested bool, stdout, stderr *os.File) (handled bool, exitCode int) {
	for _, arg := range args {
		if arg != "--help" && arg != "-h" {
			continue
		}
		var buffer bytes.Buffer
		visible := flagsListedInHelp(flags)
		visible.SetOutput(&buffer)
		visible.PrintDefaults()
		if suggestRequested {
			flags.SetOutput(io.Discard)
		} else {
			flags.SetOutput(stderr)
		}
		fmt.Fprintln(stdout, codesignalUsage)
		fmt.Fprint(stdout, buffer.String())
		return true, 0
	}
	return false, 0
}

// hiddenFromHelp holds flags that are accepted but not advertised until they
// graduate from experimental: the user documentation describes them only once
// they are supported.
var hiddenFromHelp = map[string]bool{"worktree": true}

// flagsListedInHelp copies every advertised flag into a fresh set so
// PrintDefaults keeps the standard formatting without the hidden ones.
func flagsListedInHelp(flags *flag.FlagSet) *flag.FlagSet {
	listed := flag.NewFlagSet(flags.Name(), flag.ContinueOnError)
	flags.VisitAll(func(f *flag.Flag) {
		if !hiddenFromHelp[f.Name] {
			listed.Var(f.Value, f.Name, f.Usage)
		}
	})
	return listed
}

// printUsageWithoutHiddenFlags is the usage the flag package prints after a
// flag error, in place of its default, which would list the hidden flags.
func printUsageWithoutHiddenFlags(flags *flag.FlagSet) {
	fmt.Fprintf(flags.Output(), "Usage of %s:\n", flags.Name())
	listed := flagsListedInHelp(flags)
	listed.SetOutput(flags.Output())
	listed.PrintDefaults()
}
