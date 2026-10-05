package codesignalcli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// PrepareCompilerRemediationWithReadiness extends PrepareCompilerRemediation
// for a gap already wrapped with its own readiness snapshot
// (CompilerUnresolvedErrorWithReadiness): gapCodeIsExecutablePrepareCompiler
// alone is not enough to promise the printed command will do anything, since
// AvailableSetupChoices can still resolve to a menu --prepare-compiler cannot
// act on. The test is what that flag would execute, not what the scan's own
// combined menu offers: RunPrepareCompilerMiseSetup discards every non-mise
// kind (filterMiseChoiceKinds, project_ts_compiler_mise_install.go), so a
// menu whose only executable entry is project_package -- an npm/pnpm/Bun
// repository that has simply never installed its declared compiler, and the
// scan's own controlling-terminal offer resolves it -- is as much a dead end
// as an empty one. Printing the command in either case would open a session,
// offer nothing, and exit 0 having changed nothing while the compiler is
// still missing -- exactly the dead end PrepareCompilerRemediation's own doc
// comment already promises never to name, and the one shape a piped CI
// operator cannot tell apart from success. readiness == nil falls back to
// PrepareCompilerRemediation's own gapCode-only decision, matching
// WrapCompilerUnresolvedErrorWithReadiness's contract of returning the
// original error unchanged when readiness itself could not be computed.
func PrepareCompilerRemediationWithReadiness(gapCode, configPath string, readiness *projectreadiness.Result) string {
	if readiness == nil {
		return PrepareCompilerRemediation(gapCode, configPath)
	}
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	if !menuOffersExecutableMiseChoice(AvailableSetupChoices(*readiness)) {
		return ""
	}
	return onATerminal(typescriptInvocation("--prepare-compiler", configPath))
}

// WrapCompilerUnresolvedErrorWithReadiness recomputes readiness for
// dir/revision/configPath and wraps err with it. It returns err unchanged
// when err is not a *tstoolchain.CompilerUnresolvedError, or when readiness itself
// cannot be computed: an unofferable setup session is a strictly smaller
// problem than losing the original diagnostic entirely. tstoolchain.CompilerUnresolvedError
// is only ever constructed deep inside the TypeScript project backend, so
// recomputing readiness here -- rather than threading a precomputed snapshot
// down through that backend -- is what lets this wrapping live entirely in
// this file.
func WrapCompilerUnresolvedErrorWithReadiness(err error, dir, revision, configPath string) error {
	var unresolved *tstoolchain.CompilerUnresolvedError
	if !errors.As(err, &unresolved) {
		return err
	}
	readiness, readinessErr := projectcheck.Run(dir, revision, configPath)
	if readinessErr != nil {
		return err
	}
	return &CompilerUnresolvedErrorWithReadiness{CompilerUnresolvedError: unresolved, Readiness: readiness, Revision: revision}
}

// runProjectPackageSetupOffer executes SetupChoiceProjectPackage through its
// own library path (BuildSetupPreview/RunConfirmedSetupAndRecheckReadiness),
// distinct from runMiseSetupOffer's mise install path.
func runProjectPackageSetupOffer(ctx context.Context, dir, revision, configPath, workingDirectory string, packageManager projectreadiness.Check, out io.Writer, reader *bufio.Reader) CompilerSetupOfferResult {
	preview, err := BuildSetupPreview(SetupChoice{Kind: SetupChoiceProjectPackage}, packageManager, workingDirectory)
	if err != nil {
		return CompilerSetupOfferResult{Choice: SetupChoiceProjectPackage, FailureDetail: err.Error()}
	}
	printSetupPreview(out, preview)
	confirmed := promptForSetupConfirmation(out, reader)

	outcome, execErr := RunConfirmedSetupAndRecheckReadiness(ctx, preview, confirmed, dir, revision, configPath)
	result := CompilerSetupOfferResult{
		Choice:               SetupChoiceProjectPackage,
		ChangedPaths:         repositoryRelativeChangedPaths(dir, outcome.ChangedPaths, outcome.ResidueUnknown),
		ResidueUnknown:       outcome.ResidueUnknown,
		PostInstallReadiness: outcome.PostInstallReadiness,
	}
	switch outcome.Kind {
	case SetupOutcomeCancelled:
		result.Cancelled = true
	case SetupOutcomeSucceeded:
		result.Succeeded = true
	default:
		result.FailureDetail = projectPackageSetupFailureDetail(outcome, execErr)
	}
	return result
}

func projectPackageSetupFailureDetail(outcome SetupOutcome, err error) string {
	if err != nil {
		return err.Error()
	}
	if outcome.Execution.TimedOut {
		return fmt.Sprintf("%s %s timed out", outcome.Execution.Executable, strings.Join(outcome.Execution.Args, " "))
	}
	return fmt.Sprintf("%s %s exited %d", outcome.Execution.Executable, strings.Join(outcome.Execution.Args, " "), outcome.Execution.ExitCode)
}

// PrepareCompilerRemediation names the interactive, consented mise
// TypeScript compiler-setup command that resolves a tstoolchain.CompilerUnresolvedError
// gap, for AC-SET-9's appended no-controlling-terminal remediation line. It
// returns "" for a gap code whose next action is not the executable
// prepare-compiler kind (e.g. node_missing, node_unsupported): Coach has no
// setup command that fixes a runtime-boundary gap, so appending one would
// name a command that either does nothing or targets the wrong problem.
func PrepareCompilerRemediation(gapCode, configPath string) string {
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	return onATerminal(typescriptInvocation("--prepare-compiler", configPath))
}
