package gitrepo

import (
	"strconv"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// TerminalSafe is text that git printed, made safe to put in a message that reaches a terminal. Git turns an
// ASCII control character into "?", but a right-to-left override (U+202E) and the 8-bit CSI (U+009B) pass
// through it, and a terminal acts on both; a repository chooses the names and the configuration git quotes.
// Text that shellword.Printable accepts is returned as it is, so ordinary git words stay readable; anything
// else is shown in Go syntax, so each such character is visible as an escape and none reaches the terminal.
func TerminalSafe(text string) string {
	if shellword.Printable(text) {
		return text
	}
	return strconv.Quote(text)
}
