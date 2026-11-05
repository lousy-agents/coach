// Package invocation writes the `coach codesignal` commands that a remediation
// line asks a user to run, so the check, the compiler setup and the scan name a
// policy the same way.
package invocation

import "github.com/lousy-agents/coach/internal/codesignalcli/shellword"

// Mode is the flag that selects what the TypeScript command does. The scan
// itself has none.
type Mode string

const (
	Scan                 Mode = ""
	CheckProject         Mode = "--check-project"
	PrepareCompiler      Mode = "--prepare-compiler"
	SuggestProjectConfig Mode = "--suggest-project-config"
)

// Command is one TypeScript `coach codesignal --baseline` invocation.
type Command struct {
	Mode Mode
	// ConfigPath is the --project-config the user passed, empty when the policy
	// was discovered.
	ConfigPath string
}

// Phrase is the command as the end of a line, led by verb unless verb is empty.
//
// A policy the user named whose name cannot be shown on a terminal gets no
// command at all. The same command without the flag is a valid command that
// checks the discovered policy instead, so a user who pastes it learns about
// the wrong file; the phrase says to name the same policy instead.
func (c Command) Phrase(verb string) string {
	if c.ConfigPath != "" && !shellword.Printable(c.ConfigPath) {
		return c.withheld()
	}
	if verb == "" {
		return c.text()
	}
	return verb + " " + c.text()
}

func (c Command) text() string {
	policy := ""
	if c.ConfigPath != "" {
		policy = " --project-config " + shellword.Word(c.ConfigPath)
	}
	if c.Mode == Scan {
		return "coach codesignal --baseline" + policy + " --project-language typescript"
	}
	return "coach codesignal --baseline " + string(c.Mode) + " --project-language typescript" + policy
}

func (c Command) withheld() string {
	const samePolicy = "the same --project-config (its name is not printed here)"
	switch c.Mode {
	case CheckProject:
		return "check it with --check-project and " + samePolicy
	case PrepareCompiler:
		return "run --prepare-compiler with " + samePolicy
	}
	return "rerun this scan with " + samePolicy
}
