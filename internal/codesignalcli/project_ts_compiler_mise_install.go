package codesignalcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// PrepareCompilerMiseResult reports one interactive prepare_compiler mise
// setup session's outcome (coach#328 Task 5, AC-SET-1..AC-SET-8).
//
// NoChoicesOffered, Cancelled, and a completed install attempt
// (Trusted/Attempted/Succeeded) are three separate, non-overlapping
// outcomes, mirroring AuthoringResult's Approved/Cancelled split: there was
// nothing to set up, the user declined to proceed, and an install actually
// ran, are distinct facts a caller must not conflate when deciding whether
// anything was mutated or reported.
type PrepareCompilerMiseResult struct {
	// NoChoicesOffered is true when the prepare_compiler next action was
	// absent, not executable, or named no mise installation choice at all --
	// there was nothing to set up. This is not a failure and not a
	// cancellation: no prompt was ever shown.
	NoChoicesOffered bool

	// Cancelled is true when the user declined an explicit choice selection
	// or the single-use install confirmation, including any unrecognized
	// answer (AC-SET-8, AC-SET-5: no default is ever assumed). No install
	// was attempted when this is true.
	Cancelled bool

	// Choice is the mise origin the user selected ("mise_project" or
	// "mise_global"), set once selection completed regardless of what
	// happened afterward.
	Choice string

	// Trusted, Attempted, Observed, Succeeded, Code, Origin, Class, and
	// NativePath mirror miseInstallResult's own fields for the install that
	// was actually run. They are all zero when Cancelled or NoChoicesOffered
	// is true: an install is only ever attempted after selection and
	// confirmation both completed.
	//
	// Class and Observed together let a caller distinguish a verification
	// failure (Observed true, the subprocess itself exited 0, but Class
	// names an ineligible classifyCompilerCandidate result) from an actual
	// install failure (Observed false, or the subprocess's own exit was
	// non-zero) -- Class is empty in the latter case, since
	// classifyCompilerCandidate never ran.
	Trusted    bool
	Attempted  bool
	Observed   bool
	Succeeded  bool
	Code       string
	Origin     string
	Class      string
	NativePath string

	// Version mirrors miseInstallResult.Version when an install outcome was
	// actually observed (installMiseTypescript populates it from the
	// freshly-installed candidate); otherwise it falls back to the
	// requested TypeScript version, so a failure message can always name
	// the tool spec that may have been partially written even when no
	// installed version was ever observed (AC-SET-7).
	Version string

	// PostInstallReadiness holds the rerun readiness result computed after a
	// successful install (AC-SET-6), nil otherwise.
	PostInstallReadiness *ReadinessResult

	// PostInstallReadinessError holds a failure rerunning readiness after an
	// otherwise-successful install. The install itself already succeeded and
	// is not rolled back; this only means the fresh readiness verdict could
	// not be computed.
	PostInstallReadinessError error
}

// RunPrepareCompilerMiseSetup runs the interactive prepare_compiler mise
// setup session over in/out: present the offered mise installation choices,
// require an explicit selection with no default (AC-SET-5), show the full
// AC-SET-2 preview for that choice, require single-use explicit confirmation
// (AC-SET-3), then run the matching installMiseTypescriptProject/Global and,
// on success, rerun CheckProjectReadiness (AC-SET-6). readiness must be the
// caller's already-computed ReadinessResult for dir/revision/configPath; it
// is never recomputed here except after a successful install.
//
// It never itself checks for a controlling terminal -- that gate belongs to
// the CLI caller, before readiness is even computed, mirroring
// runAuthorProjectConfigTypeScript/authorProjectConfigTypeScript's split.
func RunPrepareCompilerMiseSetup(ctx context.Context, dir, revision, configPath string, readiness *ReadinessResult, in io.Reader, out io.Writer) PrepareCompilerMiseResult {
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

	version := newestSupportedTypescriptVersion()
	worktreeRoot := compilerWorktreeRoot(dir)
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

func prepareCompilerNextAction(readiness *ReadinessResult) (ReadinessNextAction, bool) {
	if readiness == nil {
		return ReadinessNextAction{}, false
	}
	for _, action := range readiness.NextActions {
		if action.Kind == nextActionKindPrepareCompiler {
			return action, true
		}
	}
	return ReadinessNextAction{}, false
}

// miseSetupChoicesForReadiness recomputes evaluateMiseSetupChoices' result
// for dir/revision/configPath, reusing checkPolicy exactly the way
// CheckProjectReadiness itself derives roots -- never a second, independent
// notion of "roots".
func miseSetupChoicesForReadiness(dir, revision, configPath string) []ReadinessMiseChoice {
	policyPath := configPath
	if policyPath == "" {
		policyPath = defaultProjectConfigPath
	}
	_, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return nil
	}
	return evaluateMiseSetupChoices(dir, roots)
}

// miseChoicesForPrepareCompiler names which of mise_project/mise_global are
// genuinely offered for action, consuming rather than re-deriving
// CheckProjectReadiness' own verification data (AC-22): action.Choices when
// SA-280-045's package-manager-adapter restriction already populated it, or
// each mise scope's own ReadinessMiseChoice.Verified
// (miseSetupChoicesForReadiness, the same evaluateMiseSetupChoices call
// CheckProjectReadiness itself makes) when it did not -- action.Choices is
// nil exactly when no adapter rejection has restricted it yet, not when
// nothing is offered (see restrictPrepareCompilerChoices' own contract).
func miseChoicesForPrepareCompiler(dir, revision, configPath string, action ReadinessNextAction) []string {
	if action.Choices != nil {
		return filterMiseChoiceKinds(action.Choices)
	}
	var offered []string
	for _, choice := range miseSetupChoicesForReadiness(dir, revision, configPath) {
		if choice.Verified {
			offered = append(offered, choice.Kind)
		}
	}
	return offered
}

// filterMiseChoiceKinds keeps only the mise origins this flow handles.
// action.Choices may also name a project-package-manager choice from a
// sibling task; that choice is a different installation-choice kind
// entirely (SA-280-045's PackageManagerKind), not this flow's concern.
func filterMiseChoiceKinds(choices []string) []string {
	var mise []string
	for _, choice := range choices {
		if choice == compilerOriginMiseProject || choice == compilerOriginMiseGlobal {
			mise = append(mise, choice)
		}
	}
	return mise
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

// promptForMiseSetupChoice requires the user to type one offered choice's
// exact name, or "cancel". There is no numbered/default selection (AC-SET-5):
// an unrecognized or blank answer cancels rather than falling back to any
// choice, including when only one is offered.
func promptForMiseSetupChoice(out io.Writer, reader *bufio.Reader, choices []string) (choice string, cancelled bool) {
	fmt.Fprintln(out, "TypeScript compiler setup: the following mise installation choices are executable and verified in this environment:")
	for _, c := range choices {
		fmt.Fprintf(out, "  - %s\n", c)
	}
	fmt.Fprintln(out, "Type the exact choice name to select it, or 'cancel' to cancel without making any change. There is no default: an unrecognized or blank answer cancels.")
	fmt.Fprint(out, "> ")
	answer, unreadable := readLine(reader)
	if unreadable {
		return "", true
	}
	answer = strings.TrimSpace(answer)
	for _, c := range choices {
		if answer == c {
			return c, false
		}
	}
	return "", true
}

// printMisePreparePreview prints the AC-SET-2 preview: executable, arguments,
// working directory, expected mise changes, network use, lifecycle-script
// policy, and timeout, describing exactly what runMiseInstallInsulated (via
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
// gate (AC-SET-3): only the exact token "install" (case-insensitive)
// proceeds; there is no retry, mirroring promptForApproval's own gate --
// the user either confirms what was just shown or they don't.
func promptForMiseInstallConfirmation(out io.Writer, reader *bufio.Reader) bool {
	fmt.Fprintln(out, "Type 'install' to run this setup command now, or anything else to cancel without making any change:")
	fmt.Fprint(out, "> ")
	answer, _ := readLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "install")
}
