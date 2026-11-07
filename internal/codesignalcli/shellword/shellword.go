// Package shellword writes text into a command a user will paste into a POSIX
// shell, and decides whether it can be shown on a terminal at all.
package shellword

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Printable reports whether text can be shown on a terminal or a line reader
// without being reinterpreted, because quoting only protects against a shell.
// It rejects bytes that are not valid UTF-8 (a terminal may read a stray 0x9b as
// a C1 control) and every rune in Cc (C0, DEL and the C1 range, where U+009B is
// an 8-bit CSI), Cf, Zl or Zp. Control characters can drive the terminal. Cf
// holds the bidi controls, which reorder the printed command, and the
// zero-width and other format characters (zero-width space, word joiner, byte
// order mark, soft hyphen, tag characters), which alter what a reader sees. Zl
// and Zp (U+2028, U+2029) break the one-line contract in line readers and log
// viewers.
//
// This is a category filter, not an invisibility check: invisible characters
// outside these categories, such as U+034F, U+FE0F, U+E0100 (Mn) and U+3164,
// U+115F (Lo), pass, and so do the no-break space and private-use characters.
func Printable(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	return strings.IndexFunc(text, func(r rune) bool {
		return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp)
	}) < 0
}

// plainWord holds only characters no shell treats specially in a word. A
// leading "=" is excluded because zsh expands "=cmd" to cmd's path, a tilde is
// expanded to a home directory at the start of a word, and a caret is a history
// substitution or a pattern negation in some shells, so none of them is plain
// where it could be read that way.
var plainWord = regexp.MustCompile(`^[A-Za-z0-9@%+:,./_-][A-Za-z0-9@%+=:,./_-]*$`)

// Word is text as one shell word: bare when it holds only characters a shell
// leaves alone, quoted otherwise.
func Word(text string) string {
	if plainWord.MatchString(text) {
		return text
	}
	return Quote(text)
}

// Join is words as one shell command line.
func Join(words []string) string {
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = Word(word)
	}
	return strings.Join(quoted, " ")
}

// Quote single-quotes text, the only form a POSIX shell expands nothing in.
func Quote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}
