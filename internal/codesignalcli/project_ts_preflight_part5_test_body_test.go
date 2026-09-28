package codesignalcli

import (
	"slices"
	"strings"
	"testing"
)

func body_projectTsPreflightPart5Test_withoutAControllingTerminalPrintsTheRemediationL_15(t *testing.T, line string) {
	if got := AppendedRemediationLine(false, "typescript", line); got != line {
		t.Fatalf("AppendedRemediationLine(false, %q, line) = %q, want %q", "typescript", got, line)
	}
}

func body_projectTsPreflightPart5Test_aTypeScriptControllingTerminalWithholdsTheLineBe_20(t *testing.T, line string) {
	if got := AppendedRemediationLine(true, "typescript", line); got != "" {
		t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want empty: the interactive setup offer owns the controlling-terminal case for typescript", "typescript", got)
	}
}

func body_projectTsPreflightPart5Test_aGoControllingTerminalStillPrintsTheLineBecauseG_25(t *testing.T, line string) {
	if got := AppendedRemediationLine(true, "go", line); got != line {
		t.Fatalf("AppendedRemediationLine(true, %q, line) = %q, want %q: go has no interactive setup offer to own the controlling-terminal case", "go", got, line)
	}
}

func body_projectTsPreflightPart5Test_TypeScriptNamesTheGuidedAuthoringInvocationPlusT_33(t *testing.T) {
	ts := SuggestProjectConfigRemediation("typescript")
	if !strings.Contains(ts, "coach codesignal --baseline --suggest-project-config --project-language typescript") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to still name the guided-authoring invocation", "typescript", ts)
	}
	if !strings.Contains(ts, "requires a controlling terminal") || !strings.Contains(ts, "draft the schema-1 project-config document yourself") {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want it to state the terminal requirement and the no-terminal path, since this line is printed only when no controlling terminal is available", "typescript", ts)
	}
}

func body_projectTsPreflightPart5Test_GoIsOfferedOnlyTheNonInteractiveSuggestCommand_42(t *testing.T) {
	if got, want := SuggestProjectConfigRemediation("go"), "coach codesignal --baseline --suggest-project-config"; got != want {
		t.Fatalf("SuggestProjectConfigRemediation(%q) = %q, want %q: a go scan must never be offered the TypeScript-only guided-authoring command (AC-2)", "go", got, want)
	}
}

func body_projectTsPreflightPart5Test_107(t *testing.T, root string, c struct {
	name           string
	changedPaths   []string
	residueUnknown bool
	want           []string
}) {
	got := repositoryRelativeChangedPaths(root, c.changedPaths, c.residueUnknown)
	if !slices.Equal(got, c.want) {
		t.Fatalf("repositoryRelativeChangedPaths(%q, %v, %v) = %v, want %v", root, c.changedPaths, c.residueUnknown, got, c.want)
	}
}
