package codesignalcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

// PrepareCompilerMiseResult reports one interactive prepare_compiler mise
// setup session's outcome. NoChoicesOffered, Cancelled, and a completed
// install attempt (Trusted/Attempted/Succeeded) are three separate,
// non-overlapping outcomes: there was nothing to set up, the user declined
// to proceed, and an install actually ran, are distinct facts a caller must
// not conflate when deciding whether anything was mutated or reported.
type PrepareCompilerMiseResult struct {
	// PolicyRequired is true while checks.Policy has not passed: guided
	// policy authoring is the required first interactive action while a
	// policy gap and a compiler gap coexist, so compiler setup is never
	// offered -- no choice is ever listed and no prompt is ever shown.
	// aggregateReadiness itself still reports prepare_compiler as executable
	// in this case; this flow is the point that actually withholds the
	// choice.
	PolicyRequired bool

	// NoChoicesOffered is true when the prepare_compiler next action was
	// absent, not executable, or named no mise installation choice at all --
	// there was nothing to set up. This is not a failure and not a
	// cancellation: no prompt was ever shown.
	NoChoicesOffered bool

	// Cancelled is true when the user declined an explicit choice selection
	// or the single-use install confirmation, including any unrecognized
	// answer: no default is ever assumed. No install was attempted when
	// this is true.
	Cancelled bool

	// Choice is the mise origin the user selected ("mise_project" or
	// "mise_global"), set once selection completed regardless of what
	// happened afterward.
	Choice string

	// Trusted, Attempted, Observed, Succeeded, Code, Origin, Class, and
	// NativePath mirror miseInstallResult's own fields for the install that
	// was actually run. They are all zero when Cancelled or NoChoicesOffered
	// is true. VerificationFailed, AttemptFailed, and NeverStarted classify
	// a !Succeeded outcome.
	Trusted    bool
	Attempted  bool
	Observed   bool
	Succeeded  bool
	Code       string
	Origin     string
	Class      string
	NativePath string

	// Version mirrors miseInstallResult.Version when an install outcome was
	// actually observed; otherwise it falls back to the requested
	// TypeScript version, so a failure message can always name the tool
	// spec that may have been partially written even when no installed
	// version was ever observed.
	Version string

	// PostInstallReadiness holds the rerun readiness result computed after a
	// successful install, nil otherwise.
	PostInstallReadiness *projectreadiness.Result

	// PostInstallReadinessError holds a failure rerunning readiness after an
	// otherwise-successful install. The install itself already succeeded and
	// is not rolled back; this only means the fresh readiness verdict could
	// not be computed.
	PostInstallReadinessError error

	// RuntimeGapCode is set when readiness.Checks.Runtime independently fails
	// with a gap this flow has no setup command for (node_missing,
	// node_unsupported): a runtime-boundary gap takes priority over a
	// simultaneously offerable prepare_compiler action, mirroring the real
	// scan's own priority (PrepareTSRuntime resolves host Node before ever
	// attempting compiler resolution). No prompt is ever shown when this is
	// set.
	RuntimeGapCode string
}

// VerificationFailed reports whether the install subprocess itself exited
// 0, but the resulting TypeScript install did not classify as eligible.
func (r PrepareCompilerMiseResult) VerificationFailed() bool {
	return r.Observed && r.Class != ""
}

// AttemptFailed reports whether a just-run install was observed to time
// out, overflow its output budget, or exit non-zero.
func (r PrepareCompilerMiseResult) AttemptFailed() bool {
	return r.Attempted
}

// NeverStarted reports whether the install could not even be started (mise
// absent from PATH, or its own insulation could not be established).
func (r PrepareCompilerMiseResult) NeverStarted() bool {
	return r.Code != ""
}

// RunPrepareCompilerMiseSetup runs the interactive prepare_compiler mise
// setup session over in/out: present the offered mise installation choices,
// require an explicit selection with no default, show the full preview for
// that choice, require single-use explicit confirmation, then run the
// matching installMiseTypescriptProject/Global and, on success, rerun
// CheckProjectReadiness. readiness must be the caller's already-computed
// projectreadiness.Result for dir/revision/configPath; it is never recomputed here
// except after a successful install.
//
// It never itself checks for a controlling terminal -- that gate belongs to
// the CLI caller, before readiness is even computed, mirroring
// runAuthorProjectConfigTypeScript/authorProjectConfigTypeScript's split.
func RunPrepareCompilerMiseSetup(ctx context.Context, dir, revision, configPath string, readiness *projectreadiness.Result, in io.Reader, out io.Writer) PrepareCompilerMiseResult {
	if readiness != nil && readiness.Checks.Policy.State != projectreadiness.Pass {
		return PrepareCompilerMiseResult{PolicyRequired: true}
	}
	if code, blocking := readinessHasBlockingRuntimeGap(readiness); blocking {
		return PrepareCompilerMiseResult{RuntimeGapCode: code}
	}
	action, ok := prepareCompilerNextAction(readiness)
	if !ok || !action.Executable {
		return PrepareCompilerMiseResult{NoChoicesOffered: true}
	}
	choices := miseChoicesForPrepareCompiler(dir, revision, configPath, action)
	if len(choices) == 0 {
		return PrepareCompilerMiseResult{NoChoicesOffered: true}
	}

	reader := bufio.NewReader(in)
	choice, cancelled := promptForMiseSetupChoice(out, reader, choices)
	if cancelled {
		return PrepareCompilerMiseResult{Cancelled: true}
	}

	worktreeRoot := compilerWorktreeRoot(dir)
	version, ok := miseScopeDeclaresInstallableCompiler(choice, worktreeRoot)
	if !ok {
		return PrepareCompilerMiseResult{Choice: choice}
	}
	printMisePreparePreview(out, choice, version)

	if !promptForMiseInstallConfirmation(out, reader) {
		return PrepareCompilerMiseResult{Cancelled: true, Choice: choice}
	}

	installed := runSelectedMiseInstall(ctx, choice, worktreeRoot, version)
	resultVersion := installed.Version
	if resultVersion == "" {
		resultVersion = version
	}
	outcome := PrepareCompilerMiseResult{
		Choice:     choice,
		Trusted:    installed.Trusted,
		Attempted:  installed.Attempted,
		Observed:   installed.Observed,
		Succeeded:  installed.Succeeded,
		Code:       installed.Code,
		Origin:     installed.Origin,
		Class:      installed.Class,
		NativePath: installed.NativePath,
		Version:    resultVersion,
	}
	if !installed.Trusted || !installed.Succeeded {
		return outcome
	}

	postInstall, err := CheckProjectReadiness(dir, revision, configPath)
	outcome.PostInstallReadiness = postInstall
	outcome.PostInstallReadinessError = err
	return outcome
}

func runSelectedMiseInstall(ctx context.Context, choice, worktreeRoot, version string) miseInstallResult {
	switch choice {
	case compilerOriginMiseProject:
		return installMiseTypescriptProject(ctx, worktreeRoot, version)
	case compilerOriginMiseGlobal:
		return installMiseTypescriptGlobal(ctx, version)
	default:
		return miseInstallResult{}
	}
}

// printMisePreparePreview prints the preview: executable, arguments, working
// directory, expected mise changes, network use, lifecycle-script policy,
// and timeout, describing exactly what runMiseInstallInsulated (via
// installMiseTypescriptProject/Global) actually does -- never a different,
// aspirational behavior.
func printMisePreparePreview(out io.Writer, choice, version string) {
	toolSpec := miseInstallToolSpec(version)
	fmt.Fprintln(out, "Before this setup command runs, here is exactly what it will do:")
	fmt.Fprintf(out, "  Selected mise scope: %s\n", choice)
	fmt.Fprintln(out, "  Executable: mise")
	fmt.Fprintf(out, "  Arguments: install %s; then mise where %s to locate the installed compiler\n", toolSpec, toolSpec)
	fmt.Fprintln(out, "  Working directory: a private, freshly created directory outside this repository -- mise never discovers this repository's own configuration while installing")
	fmt.Fprintf(out, "  Expected mise changes: installs TypeScript %s into mise's shared tool store (via `mise install`); does not modify this project's mise.toml or any global mise configuration file\n", version)
	fmt.Fprintf(out, "  Network use: yes -- mise downloads %s from the npm registry\n", toolSpec)
	fmt.Fprintln(out, "  Lifecycle-script policy: lifecycle scripts are suppressed for the backing npm invocation (npm_config_ignore_scripts=true); mise's own current default npm backend, and its npm.shell_out=true fallback, also suppress them independently today")
	fmt.Fprintf(out, "  Timeout: %s\n", miseInstallTimeout)
}

// promptForMiseInstallConfirmation is the single-use explicit confirmation
// gate: only the exact token "install" (case-insensitive) proceeds; there
// is no retry, mirroring promptForApproval's own gate -- the user either
// confirms what was just shown or they don't.
func promptForMiseInstallConfirmation(out io.Writer, reader *bufio.Reader) bool {
	fmt.Fprintln(out, "Type 'install' to run this setup command now, or anything else to cancel without making any change:")
	fmt.Fprint(out, "> ")
	answer, _ := prompt.ReadLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "install")
}
