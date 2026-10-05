package codesignalcli

import (
	"errors"
	"path/filepath"
)

// ScanSetupOfferRemediation names the interactive scan invocation itself
// (R2) when a compiler gap's only genuinely executable setup choice is
// project_package: --prepare-compiler can never resolve that choice, since
// RunPrepareCompilerMiseSetup discards every non-mise kind
// (project_ts_compiler_mise_install.go), so
// PrepareCompilerRemediationWithReadiness withholds its own command for
// exactly this menu. Without this, a no-TTY invocation whose only path
// forward is project_package was told nothing beyond the bare
// --check-project line, even though rerunning the same scan on a terminal
// would open the combined setup offer (RunCompilerSetupOffer) and resolve
// it. It returns "" whenever PrepareCompilerRemediationWithReadiness would
// already offer its own command (a verified mise choice exists too), so the
// two remediations are never both printed for the same gap.
func ScanSetupOfferRemediation(gapCode, configPath string, readiness *ReadinessResult) string {
	if readiness == nil || !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	menu := AvailableSetupChoices(*readiness)
	if !menuOffersExecutableChoice(menu) || menuOffersExecutableMiseChoice(menu) {
		return ""
	}
	return onATerminal(typescriptScanInvocation(configPath) + " -- offers project-package setup")
}

// WrapProjectConfigErrorWithReadiness recomputes readiness for
// dir/revision/configPath and wraps err with it, for AC-SET-13's
// report-all-gaps requirement. It returns err unchanged when err is not a
// *ProjectConfigError, or when readiness itself cannot be computed: a masked
// compiler gap is a strictly smaller problem than losing the original
// diagnostic entirely.
func WrapProjectConfigErrorWithReadiness(err error, dir, revision, configPath string) error {
	var configErr *ProjectConfigError
	if !errors.As(err, &configErr) {
		return err
	}
	readiness, readinessErr := CheckProjectReadiness(dir, revision, configPath)
	if readinessErr != nil {
		return err
	}
	return &ProjectConfigErrorWithReadiness{ProjectConfigError: configErr, Readiness: readiness, ConfigPath: configPath}
}

func menuOffersChoice(menu SetupChoiceMenu, kind SetupChoiceKind) bool {
	for _, choice := range menu.Choices {
		if choice.Kind == kind {
			return true
		}
	}
	return false
}

// repositoryRelativeChangedPaths renders outcome.ChangedPaths for display.
// Ordinarily they are already repository-root-relative (RunConfirmedSetup's
// own `git status` read), but setupResidueChangedPaths' documented fallback
// names workingDirectory itself as an absolute path when residueUnknown is
// true -- rewriting that single entry relative to the worktree root keeps
// every path this function returns repository-relative, never leaking an
// absolute filesystem path to the customer.
func repositoryRelativeChangedPaths(dir string, changedPaths []string, residueUnknown bool) []string {
	if !residueUnknown || len(changedPaths) != 1 {
		return changedPaths
	}
	root := compilerWorktreeRoot(dir)
	rel, err := filepath.Rel(root, changedPaths[0])
	if err != nil {
		return changedPaths
	}
	return []string{rel}
}

// menuOffersExecutableChoice reports whether menu carries at least one
// non-cancel choice. AvailableSetupChoices always appends SetupChoiceCancel
// (project_ts_setup_choice.go), so len(menu.Choices) == 0 is true only when
// readiness's compiler check is not actually failing; a compiler gap with
// nothing installable still produces a one-entry (cancel-only) menu, which
// must not open an interactive prompt that can only ever be cancelled.
func menuOffersExecutableChoice(menu SetupChoiceMenu) bool {
	for _, choice := range menu.Choices {
		if choice.Kind != SetupChoiceCancel {
			return true
		}
	}
	return false
}
