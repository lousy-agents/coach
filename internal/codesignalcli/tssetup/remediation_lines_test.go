package tssetup

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

func TestAppendedRemediationLine(t *testing.T) {
	const line = "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"

	t.Run("without a controlling terminal prints the remediation line", func(t *testing.T) {
		withoutControllingTerminalPrintsRemediationLine(t, line)
	})
	t.Run("a TypeScript controlling terminal withholds the line because the interactive offer owns it", func(t *testing.T) {
		aTypeScriptControllingTerminalWithholdsLineBecauseInteractive(t, line)
	})
	t.Run("a Go controlling terminal still prints the line because Go has no interactive offer", func(t *testing.T) {
		aGoControllingTerminalStillPrintsLineBecause(t, line)
	})
}

func TestSuggestProjectConfigRemediation(t *testing.T) {
	t.Run("TypeScript names the guided-authoring invocation plus the no-terminal path", typeScriptNamesGuidedAuthoringInvocationPlusNoTerminal)
	t.Run("Go is offered only the non-interactive suggest command", goOfferedOnlyNonInteractiveSuggestCommand)
}

func withoutControllingTerminalPrintsRemediationLine(t *testing.T, line string) {
	if got := AppendedRemediationLine(false, "typescript", line); got != line {
		t.Fatalf("AppendedRemediationLine(false, %q, line) = %q, want %q", "typescript", got, line)
	}
}

func aTypeScriptControllingTerminalWithholdsLineBecauseInteractive(t *testing.T, line string) {
	if got := AppendedRemediationLine(true, "typescript", line); got != "" {
		t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want empty: the interactive setup offer owns the controlling-terminal case for typescript", "typescript", got)
	}
}

func aGoControllingTerminalStillPrintsLineBecause(t *testing.T, line string) {
	if got := AppendedRemediationLine(true, "go", line); got != line {
		t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want %q: go has no interactive setup offer to own the controlling-terminal case", "go", got, line)
	}
}

func typeScriptNamesGuidedAuthoringInvocationPlusNoTerminal(t *testing.T) {
	ts := SuggestProjectConfigRemediation("typescript", policyref.Ref{})
	if !strings.Contains(ts, "coach codesignal --baseline --suggest-project-config --project-language typescript") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to still name the guided-authoring invocation", "typescript", ts)
	}
	if !strings.Contains(ts, "requires a controlling terminal") || !strings.Contains(ts, "draft the schema-1 project-config document yourself") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to state the terminal requirement and the no-terminal path, since this line is printed only when no controlling terminal is available", "typescript", ts)
	}
}

func goOfferedOnlyNonInteractiveSuggestCommand(t *testing.T) {
	if got, want := SuggestProjectConfigRemediation("go", policyref.Ref{}), "coach codesignal --baseline --suggest-project-config"; got != want {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want %q: a go scan must never be offered the TypeScript-only guided-authoring command (AC-2)", "go", got, want)
	}
}

const (
	goAuthoring = "coach codesignal --baseline --suggest-project-config"
	tsAuthoring = "coach codesignal --baseline --suggest-project-config --project-language typescript"
)

type authoringCase struct {
	policy policyref.Ref
	want   string
}

// A policy the user named is written where they pointed: --output names it, a
// directory it needs is said to need to exist first, and the default location needs
// neither.
func TestSuggestProjectConfigRemediationForGoWritesWhereTheUserPointed(t *testing.T) {
	cases := map[string]authoringCase{
		"a named policy at the repository root":  {policyref.Named("new.json"), goAuthoring + " --output new.json"},
		"a named policy in a directory":          {policyref.Named("cfg/new.json"), "after making sure cfg exists at the repository root, run " + goAuthoring + " --output cfg/new.json"},
		"a named policy at the default location": {policyref.Named(".lousy-agents/coach.json"), goAuthoring},
		"a policy nobody named":                  {policyref.Discovered("project.json"), goAuthoring},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireRemediationEquals(t, "go", c) })
	}
}

func TestSuggestProjectConfigRemediationForTypeScriptWritesWhereTheUserPointed(t *testing.T) {
	cases := map[string]authoringCase{
		"a named policy at the repository root":  {policyref.Named("new.json"), tsAuthoring + " --output new.json -- "},
		"a named policy in a directory":          {policyref.Named("cfg/new.json"), "after making sure cfg exists at the repository root, run " + tsAuthoring + " --output cfg/new.json -- "},
		"a named policy at the default location": {policyref.Named(".lousy-agents/coach.json"), tsAuthoring + " -- "},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireRemediationContains(t, "typescript", c) })
	}
}

func requireRemediationEquals(t *testing.T, language string, c authoringCase) {
	t.Helper()
	if got := SuggestProjectConfigRemediation(language, c.policy); got != c.want {
		t.Errorf("SuggestProjectConfigRemediation(%q, %+v) = %q; want %q", language, c.policy, got, c.want)
	}
}

func requireRemediationContains(t *testing.T, language string, c authoringCase) {
	t.Helper()
	if got := SuggestProjectConfigRemediation(language, c.policy); !strings.Contains(got, c.want) {
		t.Errorf("SuggestProjectConfigRemediation(%q, %+v) = %q; want it to contain %q", language, c.policy, got, c.want)
	}
}
