package main

import (
	"flag"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var narrowingFlagNames = map[string]bool{"min-severity": true, "top": true}

// A leading "=" is excluded because zsh expands "=cmd" to cmd's path.
var shellSafeWord = regexp.MustCompile(`^[A-Za-z0-9@%+:,./_-][A-Za-z0-9@%+=:,./_-]*$`)

// seeAllCommand rebuilds the invocation that produced args with every
// narrowing flag removed. A flag that withheld nothing is dropped too: with the
// floor gone a cap could withhold what the floor had already hidden, so
// keeping it would not show everything.
//
// It returns "" when any word could be reinterpreted by a terminal or a line
// reader, because quoting only protects against a shell. That covers bytes that
// are not valid UTF-8 (a terminal may read a stray 0x9b as a C1 control) and
// every rune in Cc (C0, DEL and the C1 range, where U+009B is an 8-bit CSI),
// Cf, Zl or Zp. Cf holds the bidi controls, which reorder the printed command,
// and the invisible format characters (zero-width space, word joiner, byte
// order mark, soft hyphen, tag characters), which make the printed command
// differ from what a reader sees. Zl and Zp (U+2028, U+2029) render as a line
// break in some line readers and log viewers.
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
	if slices.ContainsFunc(words, isUnsafeToPrint) {
		return ""
	}
	return shellJoin(words)
}

func isUnsafeToPrint(word string) bool {
	return !utf8.ValidString(word) || strings.IndexFunc(word, func(r rune) bool {
		return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp)
	}) >= 0
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
