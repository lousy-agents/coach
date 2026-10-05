package codesignalcli

import (
	"bufio"
	"context"
	"io"

	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// runMiseSetupOffer executes a mise scope choice through its own library
// path (tstoolchain.MiseScopeDeclaresInstallableCompiler/installMiseTypescriptProject/
// Global, project_ts_compiler_mise_install.go/project_ts_compiler_mise_command.go),
// distinct from runProjectPackageSetupOffer's project-package path. Its
// preview and confirmation prompt are the same ones
// RunPrepareCompilerMiseSetup's standalone --prepare-compiler session uses,
// so a customer sees identical wording regardless of which flow offered the
// same mise scope.
func runMiseSetupOffer(ctx context.Context, dir, revision, configPath string, kind SetupChoiceKind, out io.Writer, reader *bufio.Reader) CompilerSetupOfferResult {
	origin := miseOriginForSetupChoiceKind(kind)
	worktreeRoot := tstoolchain.WorktreeRoot(dir)
	version, ok := tstoolchain.MiseScopeDeclaresInstallableCompiler(origin, worktreeRoot)
	if !ok {
		return CompilerSetupOfferResult{Choice: kind, FailureDetail: "the selected mise scope no longer declares an installable TypeScript version"}
	}
	printMisePreparePreview(out, origin, version)
	if !promptForMiseInstallConfirmation(out, reader) {
		return CompilerSetupOfferResult{Cancelled: true, Choice: kind}
	}

	installed := runSelectedMiseInstall(ctx, origin, worktreeRoot, version)
	result := CompilerSetupOfferResult{Choice: kind}
	if !installed.Trusted || !installed.Succeeded {
		result.FailureDetail = miseSetupOfferFailureDetail(installed, version, origin)
		return result
	}

	if postInstall, err := projectcheck.Run(dir, revision, configPath); err == nil {
		result.PostInstallReadiness = postInstall
	}
	result.Succeeded = true
	return result
}

// projectPackageWorkingDirectory resolves BuildSetupPreview's required
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

// AlsoFailingGapLines is AC-SET-13's "report all gaps" clause: one line for
// every gap readiness itself reports other than the policy failure the scan
// has already printed as its own message.
//
// It is reached only alongside that policy failure
// (ProjectConfigErrorWithReadiness is the sole production producer). Before
// R1, a missing policy left checkProjectShape and pkgmanager.Check
// guessing from the worktree root in place of the roots a policy would have
// selected, so a gap either of them raised might simply be an artifact of
// that guess -- which is why this line used to hedge rather than assert the
// gap would still be there. Both checks now report not_checked instead of
// guessing (R1), so every gap readiness.Gaps still carries here -- the
// compiler check, the runtime check, and a mise-scope trust finding -- was
// never roots-dependent in the first place, real and independent of the
// policy either way. So this simply reports readiness.Gaps unchanged, with
// no hedge left to state.
//
// readiness.Gaps is already emitted in the epic's frozen next-action order,
// so iterating it preserves that ordering rather than inventing one here.
func AlsoFailingGapLines(readiness *projectreadiness.Result, configPath string) []string {
	if readiness == nil {
		return nil
	}
	var lines []string
	seen := make(map[string]bool, len(readiness.Gaps))
	for _, gap := range readiness.Gaps {
		if gap.Code == projectreadiness.GapPolicyMissing || gap.Code == projectreadiness.GapPolicyInvalid || seen[gap.Code] {
			continue
		}
		seen[gap.Code] = true
		lines = append(lines, alsoFailingGapLine(gap.Code, configPath))
	}
	return lines
}
