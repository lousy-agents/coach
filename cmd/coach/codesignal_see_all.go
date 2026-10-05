package main

import (
	"flag"
	"regexp"
	"strings"
)

var narrowingFlagNames = map[string]bool{"min-severity": true, "top": true}

// A leading "=" is excluded because zsh expands "=cmd" to cmd's path.
var shellSafeWord = regexp.MustCompile(`^[A-Za-z0-9@%+:,./_-][A-Za-z0-9@%+=:,./_-]*$`)

// seeAllCommand rebuilds the invocation that produced args with every
// narrowing flag removed. A flag that withheld nothing is dropped too: with the
// floor gone a cap could withhold what the floor had already hidden, so
// keeping it would not show everything.
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
	return shellJoin(words)
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

func shellJoin(words []string) string {
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = shellQuote(word)
	}
	return strings.Join(quoted, " ")
}

func shellQuote(word string) string {
	if shellSafeWord.MatchString(word) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
