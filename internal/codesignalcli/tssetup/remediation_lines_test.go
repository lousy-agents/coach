package tssetup

import (
	"strings"
	"testing"
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
	ts := SuggestProjectConfigRemediation("typescript")
	if !strings.Contains(ts, "coach codesignal --baseline --suggest-project-config --project-language typescript") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to still name the guided-authoring invocation", "typescript", ts)
	}
	if !strings.Contains(ts, "requires a controlling terminal") || !strings.Contains(ts, "draft the schema-1 project-config document yourself") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to state the terminal requirement and the no-terminal path, since this line is printed only when no controlling terminal is available", "typescript", ts)
	}
}

func goOfferedOnlyNonInteractiveSuggestCommand(t *testing.T) {
	if got, want := SuggestProjectConfigRemediation("go"), "coach codesignal --baseline --suggest-project-config"; got != want {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want %q: a go scan must never be offered the TypeScript-only guided-authoring command (AC-2)", "go", got, want)
	}
}
