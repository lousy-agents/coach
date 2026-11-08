package gitrepo

import (
	"strings"
	"testing"
)

// What git says reaches the terminal inside Coach's messages. Git replaces an ASCII control character with
// "?", but a right-to-left override (U+202E) and the 8-bit CSI (U+009B) pass through it, and a terminal
// acts on both. TerminalSafe shows such a character as an escape and leaves ordinary words alone.

const (
	rightToLeftOverride = "\u202e"
	eightBitCSI         = "\u009b"
)

func TestTerminalSafeLeavesPrintableTextAsItIs(t *testing.T) {
	text := "fatal: not a git repository (or any parent up to mount point /)"

	if got := TerminalSafe(text); got != text {
		t.Errorf("TerminalSafe(%q) = %q, want it unchanged", text, got)
	}
}

type unsafeText struct{ name, text string }

func (c unsafeText) run(t *testing.T) {
	got := TerminalSafe(c.text)
	if strings.Contains(got, rightToLeftOverride) || strings.Contains(got, eightBitCSI) || strings.ContainsAny(got, "\n\xff") {
		t.Errorf("TerminalSafe(%q) = %q still holds the character raw", c.text, got)
	}
}

func TestTerminalSafeShowsAControlOrFormatCharacterAsAnEscape(t *testing.T) {
	cases := []unsafeText{
		{"a right-to-left override", "fatal: bad " + rightToLeftOverride + "name"},
		{"an 8-bit CSI", "fatal: bad " + eightBitCSI + "2J name"},
		{"invalid UTF-8", "fatal: bad \xff name"},
		{"a newline", "fatal: one\nfatal: two"},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}
