package codesignalcli

import (
	"context"

	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestAppendedRemediationLine(t *testing.T) {
	const line = "on a terminal: coach codesignal --baseline --prepare-compiler --project-language typescript"

	t.Run("without a controlling terminal prints the remediation line", func(t *testing.T) {
		body_projectTsPreflightPart5Test_withoutAControllingTerminalPrintsTheRemediationL_15(t, line)
	})
	t.Run("a TypeScript controlling terminal withholds the line because the interactive offer owns it", func(t *testing.T) {
		body_projectTsPreflightPart5Test_aTypeScriptControllingTerminalWithholdsTheLineBe_20(t, line)
	})
	t.Run("a Go controlling terminal still prints the line because Go has no interactive offer", func(t *testing.T) {
		body_projectTsPreflightPart5Test_aGoControllingTerminalStillPrintsTheLineBecauseG_25(t, line)
	})
}

func TestSuggestProjectConfigRemediation(t *testing.T) {
	t.Run("TypeScript names the guided-authoring invocation plus the no-terminal path", func(t *testing.T) {
		body_projectTsPreflightPart5Test_TypeScriptNamesTheGuidedAuthoringInvocationPlusT_33(t)
	})
	t.Run("Go is offered only the non-interactive suggest command", func(t *testing.T) {
		body_projectTsPreflightPart5Test_GoIsOfferedOnlyTheNonInteractiveSuggestCommand_42(t)
	})
}

// TestRunCompilerSetupOfferCancelsOnUnreadableSelection pins the fail-closed
// default: an input stream that produces no answer at all (EOF) must cancel
// rather than falling back to any offered choice. The fixture must offer a
// genuine, non-cancel choice (an installable manifest declaration plus a
// passing package-manager check) or the prompt this pins would never open at
// all.
func TestRunCompilerSetupOfferCancelsOnUnreadableSelection(t *testing.T) {
	readiness := &ReadinessResult{
		Checks: ReadinessChecks{
			Policy:         ReadinessCheck{State: ReadinessPass},
			Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing, DeclaredVersion: SupportedTypescriptVersions[0]},
			PackageManager: ReadinessCheck{State: ReadinessPass},
		},
	}
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)
	if !result.Cancelled {
		t.Fatalf("Cancelled = false, want true: %+v", result)
	}
	if result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if !strings.Contains(out.String(), "cancel") {
		t.Fatalf("expected the menu to have offered a cancel choice, got %q", out.String())
	}
}

// TestRepositoryRelativeChangedPathsOnlyRewritesTheResidueUnknownFallback
// pins both branches of repositoryRelativeChangedPaths: an ordinary
// git-status disclosure (already repository-relative) passes through
// unchanged, while setupResidueChangedPaths' documented residueUnknown
// fallback -- which names workingDirectory itself, an absolute path -- is
// rewritten relative to the worktree root before it can reach the customer.
func TestRepositoryRelativeChangedPathsOnlyRewritesTheResidueUnknownFallback(t *testing.T) {
	root := gitfixture.Init(t)
	workingDirectory := filepath.Join(root, "packages", "app")

	cases := []struct {
		name           string
		changedPaths   []string
		residueUnknown bool
		want           []string
	}{
		{
			name:           "a real git-status disclosure is already repository-relative and passes through unchanged",
			changedPaths:   []string{"packages/app/node_modules/"},
			residueUnknown: false,
			want:           []string{"packages/app/node_modules/"},
		},
		{
			name:           "residueUnknown's absolute workingDirectory fallback is rewritten repository-relative",
			changedPaths:   []string{workingDirectory},
			residueUnknown: true,
			want:           []string{filepath.Join("packages", "app")},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body_projectTsPreflightPart5Test_107(t, root, c)
		})
	}
}
