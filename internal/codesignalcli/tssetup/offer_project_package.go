package tssetup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// withProjectPackageResolution re-decides the project_package entry against
// the one fact AvailableChoices cannot see: which manifest context the
// install would actually run in. AvailableChoices decides purely from a
// readiness snapshot and has no dir/revision to resolve a working directory
// with, so this is where an offer that cannot be previewed honestly is
// converted into a withheld entry with its reason.
func withProjectPackageResolution(menu Menu, dir, revision, configPath string) (resolved Menu, workingDirectory, withheldReason string) {
	if !menuOffersChoice(menu, ChoiceProjectPackage) {
		return menu, "", ""
	}
	workingDirectory, withheldReason = projectPackageWorkingDirectory(dir, revision, configPath)
	if withheldReason == "" {
		return menu, workingDirectory, ""
	}
	remaining := make([]Choice, 0, len(menu.Choices))
	for _, choice := range menu.Choices {
		if choice.Kind == ChoiceProjectPackage {
			continue
		}
		remaining = append(remaining, choice)
	}
	menu.Choices = remaining
	menu.Withheld = append(menu.Withheld, WithheldChoice{Kind: ChoiceProjectPackage, Reason: withheldReason})
	return menu, "", withheldReason
}

// projectPackageWorkingDirectory resolves BuildPreview's required
// directory the same way pkgmanager.Check itself does
// (pkgmanager.Contexts): the nearest package.json above a selected policy
// root. It resolves a directory only when exactly one context exists, and
// names a withholding reason otherwise.
//
// Plurality is not a tie to break. A single consented install runs one
// previewed command in one directory, while the compiler check requires
// every selected root's manifest context to resolve the same installed
// version -- so with two contexts, whichever one is chosen, AC-SET-6's
// mandatory rerun still reports the gap. Defaulting to the first would spend
// the customer's single approval on a network install that cannot succeed,
// which is exactly the silent default AC-SET-5 forbids where ownership is
// ambiguous. An unresolvable policy is withheld for the same reason rather
// than falling back to the worktree root: the menu's eligibility was never
// computed against that directory, so running an install there would consent
// to something nobody previewed.
func projectPackageWorkingDirectory(dir, revision, configPath string) (workingDirectory, withheldReason string) {
	worktreeRoot := tstoolchain.WorktreeRoot(dir)
	policyPath := configPath
	if policyPath == "" {
		policyPath = projectconfig.DefaultPath
	}
	_, roots, err := projectcheck.CheckPolicy(dir, revision, policyPath)
	if err != nil {
		return "", setupChoiceReasonManifestContextUnresolved
	}
	contexts, _ := pkgmanager.Contexts(worktreeRoot, roots)
	switch len(contexts) {
	case 0:
		// Defensive: pkgmanager.Contexts falls back to the worktree root,
		// so it does not return an empty slice today.
		return "", setupChoiceReasonManifestContextUnresolved
	case 1:
		return contexts[0], ""
	default:
		return "", setupChoiceReasonManifestContextAmbiguous
	}
}

// runProjectPackageSetupOffer executes ChoiceProjectPackage through its
// own library path (BuildPreview/RunConfirmedAndRecheckReadiness),
// distinct from runMiseSetupOffer's mise install path.
func runProjectPackageSetupOffer(ctx context.Context, dir, revision, configPath, workingDirectory string, packageManager projectreadiness.Check, out io.Writer, reader *bufio.Reader) CompilerSetupOfferResult {
	preview, err := BuildPreview(Choice{Kind: ChoiceProjectPackage}, packageManager, workingDirectory)
	if err != nil {
		return CompilerSetupOfferResult{Choice: ChoiceProjectPackage, FailureDetail: err.Error()}
	}
	printSetupPreview(out, preview)
	confirmed := promptForSetupConfirmation(out, reader)

	outcome, execErr := RunConfirmedAndRecheckReadiness(ctx, preview, confirmed, dir, revision, configPath)
	result := CompilerSetupOfferResult{
		Choice:               ChoiceProjectPackage,
		ChangedPaths:         repositoryRelativeChangedPaths(dir, outcome.ChangedPaths, outcome.ResidueUnknown),
		ResidueUnknown:       outcome.ResidueUnknown,
		PostInstallReadiness: outcome.PostInstallReadiness,
	}
	switch outcome.Kind {
	case OutcomeCancelled:
		result.Cancelled = true
	case OutcomeSucceeded:
		result.Succeeded = true
	default:
		result.FailureDetail = projectPackageSetupFailureDetail(outcome, execErr)
	}
	return result
}

func projectPackageSetupFailureDetail(outcome Outcome, err error) string {
	if err != nil {
		return err.Error()
	}
	if outcome.Execution.TimedOut {
		return fmt.Sprintf("%s %s timed out", outcome.Execution.Executable, strings.Join(outcome.Execution.Args, " "))
	}
	return fmt.Sprintf("%s %s exited %d", outcome.Execution.Executable, strings.Join(outcome.Execution.Args, " "), outcome.Execution.ExitCode)
}
