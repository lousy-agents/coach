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
		flags.SetOutput(&buffer)
		flags.PrintDefaults()
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
