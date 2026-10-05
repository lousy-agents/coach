package tssetup

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// menuOffersExecutableMiseChoice narrows menuOffersExecutableChoice to the
// kinds the interim --prepare-compiler flag can actually run. The scan's own
// offer (RunCompilerSetupOffer) executes every kind and so uses the wider
// predicate; only a printed --prepare-compiler command needs this one.
func menuOffersExecutableMiseChoice(menu Menu) bool {
	for _, choice := range menu.Choices {
		if choice.Kind == ChoiceProjectMise || choice.Kind == ChoiceGlobalMise {
			return true
		}
	}
	return false
}

// runMiseSetupOffer executes a mise scope choice through its own library
// path (tstoolchain.MiseScopeDeclaresInstallableCompiler/installMiseTypescriptProject/
// Global, prepare_compiler.go/mise_install.go),
// distinct from runProjectPackageSetupOffer's project-package path. Its
// preview and confirmation prompt are the same ones
// RunPrepareCompilerMiseSetup's standalone --prepare-compiler session uses,
// so a customer sees identical wording regardless of which flow offered the
// same mise scope.
func runMiseSetupOffer(ctx context.Context, dir, revision, configPath string, kind ChoiceKind, out io.Writer, reader *bufio.Reader) CompilerSetupOfferResult {
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

func miseOriginForSetupChoiceKind(kind ChoiceKind) string {
	if kind == ChoiceGlobalMise {
		return tstoolchain.OriginMiseGlobal
	}
	return tstoolchain.OriginMiseProject
}

func miseSetupOfferFailureDetail(installed miseInstallResult, version, origin string) string {
	switch {
	case installed.Observed && installed.Class != "":
		return fmt.Sprintf("mise install exited 0 but the installed TypeScript %s is not eligible (%s)", version, installed.Class)
	case installed.Attempted:
		return fmt.Sprintf("mise install failed for TypeScript %s under the %s scope", version, origin)
	case installed.Code != "":
		return fmt.Sprintf("mise install could not even be started (%s)", installed.Code)
	default:
		return "mise install could not even be started"
	}
}
