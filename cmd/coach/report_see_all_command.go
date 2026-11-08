package main

import (
	"flag"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

var narrowingFlagNames = map[string]bool{"min-severity": true, "top": true}

// seeAllCommand rebuilds the invocation that produced args with every
// narrowing flag removed. A flag that withheld nothing is dropped too: with the
// floor gone a cap could withhold what the floor had already hidden, so
// keeping it would not show everything.
//
// It returns "" when any word cannot be shown as it is (see
// shellword.Printable), because quoting only protects against a shell.
func seeAllCommand(args []string) string {
	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	registerCodesignalFlags(flags)

	words := []string{"coach", "codesignal"}
	for i := 0; i < len(args); i++ {
		name, inlineValue, isFlag := splitFlagToken(args[i])
		if !isFlag {
			words = append(words, args[i:]...)
			break
		}
		span := args[i : i+1]
		if !inlineValue && flagTakesValue(flags, name) && i+1 < len(args) {
			span = args[i : i+2]
		}
		if !narrowingFlagNames[name] {
			words = append(words, span...)
		}
		i += len(span) - 1
	}
	if slices.ContainsFunc(words, func(word string) bool { return !shellword.Printable(word) }) {
		return ""
	}
	return shellword.Join(words)
}

// splitFlagToken applies the flag package's token grammar: one or two leading
// dashes, an optional =value, and "--" or a bare "-" ending the flags.
func splitFlagToken(token string) (name string, inlineValue, isFlag bool) {
	if len(token) < 2 || token[0] != '-' || token == "--" {
		return "", false, false
	}
	name = strings.TrimPrefix(strings.TrimPrefix(token, "-"), "-")
	if name == "" || name[0] == '-' || name[0] == '=' {
		return "", false, false
	}
	name, _, inlineValue = strings.Cut(name, "=")
	return name, inlineValue, true
}

func flagTakesValue(flags *flag.FlagSet, name string) bool {
	registered := flags.Lookup(name)
	if registered == nil {
		return false
	}
	boolean, ok := registered.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolean.IsBoolFlag()
}
