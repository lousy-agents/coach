package render

import (
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// terminalText is text that a repository can control, made safe to write to a terminal. The readiness
// text prints what a package.json declares, what is installed, what a policy names and the name of any
// uncommitted file, and a terminal acts on control characters in it: an OSC sequence retitles the window
// or writes the clipboard, a CSI sequence clears or rewrites the screen. Text that shellword.Printable
// accepts is returned as it is; anything else is shown in Go syntax, so each such character is visible as
// an escape ("\x1b", "\a", "\u009b") and none reaches the terminal. The JSON output needs no such step,
// because encoding/json escapes control characters itself.
func terminalText(text string) string {
	if shellword.Printable(text) {
		return text
	}
	return strconv.Quote(text)
}

// terminalList is items joined by sep, each made safe on its own, so a hostile item cannot hide the
// separator between two others.
func terminalList(items []string, sep string) string {
	safe := make([]string, len(items))
	for i, item := range items {
		safe[i] = terminalText(item)
	}
	return strings.Join(safe, sep)
}
