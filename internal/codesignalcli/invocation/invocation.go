// Package invocation writes the `coach codesignal` commands that a remediation
// line asks a user to run, so the check, the compiler setup, the scan and the
// authoring of a policy name a policy the same way.
package invocation

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

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
	// Policy is the policy the command is about. The command names it with
	// --project-config only when the user named it: naming a discovered policy
	// would make the next command judge it as a named one.
	Policy policyref.Ref
	// Target is where SuggestProjectConfig writes the candidate, named with
	// --output. It is empty for authoring's default location. Authoring rejects
	// --project-config, so a command in this mode names its policy only through
	// Target.
	Target string
}

// Phrase is the command as the end of a line, led by verb unless verb is empty. Authoring has no verb
// of its own: whatever verb it is given, it is led by the note that its target's directory must exist
// first, when there is one, and by nothing otherwise.
//
// A policy the user named whose name cannot be shown on a terminal gets no
// command at all. The same command without the flag is a valid command that
// checks the discovered policy instead, so a user who pastes it learns about
// the wrong file; the phrase says to name the same policy instead.
func (c Command) Phrase(verb string) string {
	if !c.printable() {
		return c.withheld()
	}
	if c.Mode == SuggestProjectConfig {
		verb = directoryNote(c.Target)
	}
	if verb == "" {
		return c.text()
	}
	return verb + " " + c.text()
}

// printable is false when the path the command would name cannot be shown.
func (c Command) printable() bool {
	if c.Mode == SuggestProjectConfig {
		return shellword.Printable(c.Target)
	}
	named, ok := c.Policy.NamedPath()
	return !ok || shellword.Printable(named)
}

func (c Command) text() string {
	if c.Mode == SuggestProjectConfig {
		return "coach codesignal --baseline " + string(c.Mode) + " --project-language typescript" + outputFlag(c.Target)
	}
	policy := ""
	if named, ok := c.Policy.NamedPath(); ok {
		policy = " --project-config " + shellword.Word(named)
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
	case SuggestProjectConfig:
		return samePath
	}
	return "rerun this scan with " + samePolicy
}
