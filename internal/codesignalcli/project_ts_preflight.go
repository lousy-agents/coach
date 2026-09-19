package codesignalcli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ProjectConfigErrorWithReadiness enriches a ProjectConfigError with the
// full TypeScript project-readiness snapshot computed for the same
// dir/revision/configPath. prepareProjectAnalysis's loadProjectConfig short
// circuit means only the policy failure would otherwise ever reach
// classifyAnalysisError, masking a simultaneous compiler gap (AC-SET-13).
// Unwrap returns the original *ProjectConfigError so
// errors.As(err, &plainTarget) still matches through this wrapper exactly as
// it did before wrapping existed.
type ProjectConfigErrorWithReadiness struct {
	*ProjectConfigError
	Readiness  *ReadinessResult
	ConfigPath string
}

func (e *ProjectConfigErrorWithReadiness) Unwrap() error { return e.ProjectConfigError }

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

// CompilerUnresolvedErrorWithReadiness enriches a CompilerUnresolvedError
// with the full TypeScript project-readiness snapshot computed for the same
// dir/revision/configPath, so a real scan's controlling-terminal branch can
// drive the interactive compiler-setup offer from the same facts the gap
// itself was raised from, without a second, possibly racing readiness
// computation. Unwrap returns the original
// *CompilerUnresolvedError so errors.As(err, &plainTarget) still matches
// through this wrapper exactly as it did before wrapping existed -- in
// particular, classifyAnalysisError's own no-controlling-terminal fallback
// needs no change at all.
// ConfigPath is deliberately absent: *CompilerUnresolvedError already
// carries the policy path RemediationLine() renders, and a second copy here
// could drift from it, pointing the printed remediation and the setup
// session it precedes at two different policies.
type CompilerUnresolvedErrorWithReadiness struct {
	*CompilerUnresolvedError
	Readiness *ReadinessResult
	Revision  string
}

func (e *CompilerUnresolvedErrorWithReadiness) Unwrap() error { return e.CompilerUnresolvedError }

// WrapCompilerUnresolvedErrorWithReadiness recomputes readiness for
// dir/revision/configPath and wraps err with it. It returns err unchanged
// when err is not a *CompilerUnresolvedError, or when readiness itself
// cannot be computed: an unofferable setup session is a strictly smaller
// problem than losing the original diagnostic entirely. CompilerUnresolvedError
// is only ever constructed deep inside the TypeScript project backend, so
// recomputing readiness here -- rather than threading a precomputed snapshot
// down through that backend -- is what lets this wrapping live entirely in
// this file.
func WrapCompilerUnresolvedErrorWithReadiness(err error, dir, revision, configPath string) error {
	var unresolved *CompilerUnresolvedError
	if !errors.As(err, &unresolved) {
		return err
	}
	readiness, readinessErr := CheckProjectReadiness(dir, revision, configPath)
	if readinessErr != nil {
		return err
	}
	return &CompilerUnresolvedErrorWithReadiness{CompilerUnresolvedError: unresolved, Readiness: readiness, Revision: revision}
}

// AlsoFailingGapLines is AC-SET-13's "report all gaps" clause: one line for
// every gap readiness itself reports other than the policy failure the scan
// has already printed as its own message.
//
// It is reached only alongside that policy failure
// (ProjectConfigErrorWithReadiness is the sole production producer). Before
// R1, a missing policy left checkProjectShape and checkPackageManager
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
func AlsoFailingGapLines(readiness *ReadinessResult, configPath string) []string {
	if readiness == nil {
		return nil
	}
	var lines []string
	seen := make(map[string]bool, len(readiness.Gaps))
	for _, gap := range readiness.Gaps {
		if gap.Code == GapPolicyMissing || gap.Code == GapPolicyInvalid || seen[gap.Code] {
			continue
		}
		seen[gap.Code] = true
		lines = append(lines, alsoFailingGapLine(gap.Code, configPath))
	}
	return lines
}

func alsoFailingGapLine(code, configPath string) string {
	return code + ": also failing, run " + typescriptInvocation("--check-project", configPath)
}

// PrepareCompilerRemediation names the interactive, consented mise
// TypeScript compiler-setup command that resolves a CompilerUnresolvedError
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

// onATerminal qualifies a command that refuses without a controlling
// terminal. Both commands Coach names as remediation are interactive, and
// this line is printed exactly where no terminal was available -- so a
// piped caller that runs either one verbatim gets exit 2 and no progress.
// Saying so costs a clause; discovering it costs an invocation.
func onATerminal(invocation string) string {
	return "on a terminal: " + invocation
}

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
func PrepareCompilerRemediationWithReadiness(gapCode, configPath string, readiness *ReadinessResult) string {
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

// typescriptScanInvocation names the bare `coach codesignal --baseline`
// scan itself, distinct from typescriptInvocation's own --check-project/
// --prepare-compiler forms: R2's remediation points at rerunning the
// original scan on a terminal, not at a standalone subcommand.
func typescriptScanInvocation(configPath string) string {
	invocation := "coach codesignal --baseline"
	if configPath != "" {
		invocation += " --project-config " + configPath
	}
	return invocation + " --project-language typescript"
}

// SuggestProjectConfigRemediation names the --suggest-project-config
// invocation that resolves a ProjectConfigError gap for language, for
// AC-SET-9's appended no-controlling-terminal remediation line. For
// "typescript" this is the interactive, guided policy-authoring command; for
// every other language (only "go" reaches this today) it is the plain batch
// candidate-generation command, since --project-language typescript would
// name a command that language cannot run (AC-2: an offered remediation must
// actually be supported for the language it is offered to). It never appends
// a --project-config suffix: validateSuggestProjectConfigFlags rejects
// --suggest-project-config combined with --project-config.
//
// The TypeScript form qualifies the command rather than naming it bare,
// because this line is printed precisely when no controlling terminal is
// available and guided authoring refuses without one: an agent or CI job
// that runs it verbatim gets exit 2 and no policy. The refusal's own
// instruction -- draft the document, have it reviewed and committed -- is
// the path actually open here, so it is stated alongside the command rather
// than discovered by spending an invocation on it.
func SuggestProjectConfigRemediation(language string) string {
	if language == "typescript" {
		return onATerminal(typescriptInvocation("--suggest-project-config", "")) +
			" -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"
	}
	return "coach codesignal --baseline --suggest-project-config"
}

// AppendedRemediationLine withholds line whenever hasControllingTerminal is
// true and language is "typescript": the interactive setup offer itself owns
// that case there, so AC-SET-9's appended command is printed only when no
// controlling terminal is available to run it. No such offer exists for any
// other language (only "go" reaches this today), so a controlling-terminal
// user must still see the same appended remediation a piped invocation gets.
func AppendedRemediationLine(hasControllingTerminal bool, language, line string) string {
	if hasControllingTerminal && language == "typescript" {
		return ""
	}
	return line
}

// gapCodeIsExecutablePrepareCompiler reports whether gapCode's next action,
// per the authoritative gapCodeTable, is the executable prepare-compiler
// kind -- the only kind Coach can actually run a command for
// (nextActionExecutable).
func gapCodeIsExecutablePrepareCompiler(gapCode string) bool {
	kind, ok := nextActionForGapCode(gapCode)
	return ok && nextActionExecutable(kind)
}

func typescriptInvocation(flag, projectConfigPath string) string {
	invocation := "coach codesignal --baseline " + flag + " --project-language typescript"
	if projectConfigPath != "" {
		invocation += " --project-config " + projectConfigPath
	}
	return invocation
}

// CompilerSetupOfferResult is RunCompilerSetupOffer's outcome: the
// caller-facing policy decision plus enough detail to report it, regardless
// of which of AvailableSetupChoices' kinds was actually selected and
// executed. It intentionally flattens SetupOutcome's own shape (the
// project_package execution path) and PrepareCompilerMiseResult's shape
// (the mise execution path) into one result, so a caller does not have to
// branch on which of the two ran before deciding what to report.
type CompilerSetupOfferResult struct {
	// PolicyRequired is true while readiness.Checks.Policy has not passed,
	// mirroring RunPrepareCompilerMiseSetup's own AC-SET-13 precondition
	// (project_ts_compiler_mise_install.go). The sole production caller
	// (runScanCompilerSetupOffer) only ever reaches this function downstream
	// of a successful loadProjectConfig, so this never actually fires today;
	// the guard exists so that invariant is enforced here too rather than
	// resting entirely on the caller never changing.
	PolicyRequired bool

	// NoChoicesOffered is true either when gapCode's own next action is not
	// the executable prepare-compiler kind (a runtime-boundary gap such as
	// node_missing/node_unsupported), or when readiness's compiler check
	// was not actually failing: AvailableSetupChoices returned an empty
	// menu. Either way, no prompt was ever shown.
	NoChoicesOffered bool

	// Withheld carries AvailableSetupChoices' own reasons for every
	// candidate it ruled out, but only on the NoChoicesOffered branch where
	// a menu was actually built and nothing in it was executable. That is
	// the one outcome where the customer has no next step at all -- the gap
	// line points at --check-project, which re-reports the same gap -- so
	// the reason nothing could be offered is what breaks the loop. It stays
	// nil for a runtime-boundary gap code and for a nil readiness, where no
	// menu was ever computed and there is nothing to explain.
	Withheld []WithheldSetupChoice

	// Cancelled is true when the user selected "cancel", gave an
	// unrecognized or blank answer at either prompt, or declined the
	// single-use confirmation. No setup command ever ran.
	Cancelled bool

	// Choice is the selected menu entry, set once selection completes
	// regardless of what happened afterward. It is the zero SetupChoiceKind
	// when NoChoicesOffered is true, or when the top-level selection itself
	// was cancelled.
	Choice SetupChoiceKind

	// Succeeded is true only when the confirmed install actually completed
	// (AC-SET-7): a project_package run via
	// RunConfirmedSetupAndRecheckReadiness, or a mise scope run via
	// installMiseTypescriptProject/Global.
	Succeeded bool

	// FailureDetail is a human-readable cause, populated whenever neither
	// Cancelled nor Succeeded is true: BuildSetupPreview/ExecuteSetup's own
	// refusal, a non-zero exit or timeout, a mise scope that stopped
	// declaring an installable version between menu construction and
	// confirmation, or a verification failure after an exit-0 mise install.
	FailureDetail string

	// ChangedPaths and ResidueUnknown mirror SetupOutcome's own AC-SET-7
	// disclosure for a failed project_package run; both stay nil/false for
	// every other path (mise's own installer produces no such disclosure,
	// and a cancelled or successful run has nothing to disclose).
	ChangedPaths   []string
	ResidueUnknown bool

	// PostInstallReadiness is populated only after a successful install
	// (AC-SET-6): the complete readiness rerun a caller must consult before
	// deciding whether the scan that offered this setup may continue
	// (AC-7).
	PostInstallReadiness *ReadinessResult

	// RuntimeGapCode is set when readiness.Checks.Runtime independently fails
	// with a gap this offer has no setup command for (node_missing,
	// node_unsupported). A runtime-boundary gap takes priority over a
	// simultaneously offerable prepare_compiler action, matching
	// RunPrepareCompilerMiseSetup. No prompt is shown when this is set.
	RuntimeGapCode string
}

// RunCompilerSetupOffer runs the interactive, combined compiler-setup
// session a real scan's CompilerUnresolvedError gap offers when a
// controlling terminal is available (AC-SET-9, AC-11): present
// every AvailableSetupChoices entry -- project_package and/or a verified
// mise scope -- in one menu, require an explicit single selection with no
// default, show the resolved preview, require one single-use confirmation,
// run the confirmed choice through its own execution path, and rerun
// readiness (AC-SET-6) only after a successful install.
//
// gapCode is the CompilerUnresolvedError's own Code, not
// readiness.Checks.Compiler.Code: the two are independent
// CheckProjectReadiness reads (project_readiness.go resolves Node and the
// compiler separately), so a runtime-boundary gap (node_missing,
// node_unsupported) can coexist with a readiness snapshot whose compiler
// check independently offers an installable menu entry. A blocking runtime
// check is gated first (readinessHasBlockingRuntimeGap), then
// gapCodeIsExecutablePrepareCompiler, matching RunPrepareCompilerMiseSetup
// so the scan offer and the standalone flag cannot disagree (AC-SET-10).
//
// Every prompt is read from in and every line of output written to out at
// most once per session: selection and confirmation are each read exactly
// once into a local variable that is used exactly once and then goes out of
// scope, and nothing here loops back to prompt again afterward -- so an
// answer already consumed can never be replayed against a second execution
// within the same call, and a leftover, unread answer left on in can never
// trigger one either.
//
// readiness.Checks.Policy must already be ReadinessPass, mirroring
// RunPrepareCompilerMiseSetup's own precondition: a reviewed, committed
// policy is required before compiler setup is ever offered (AC-SET-13).
func RunCompilerSetupOffer(ctx context.Context, dir, revision, configPath, gapCode string, readiness *ReadinessResult, in io.Reader, out io.Writer) CompilerSetupOfferResult {
	if readiness != nil && readiness.Checks.Policy.State != ReadinessPass {
		return CompilerSetupOfferResult{PolicyRequired: true}
	}
	if code, blocking := readinessHasBlockingRuntimeGap(readiness); blocking {
		return CompilerSetupOfferResult{RuntimeGapCode: code}
	}
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return CompilerSetupOfferResult{NoChoicesOffered: true}
	}
	if readiness == nil {
		return CompilerSetupOfferResult{NoChoicesOffered: true}
	}
	menu, projectPackageDirectory, resolutionWithheld := withProjectPackageResolution(AvailableSetupChoices(*readiness), dir, revision, configPath)
	if !menuOffersExecutableChoice(menu) {
		return CompilerSetupOfferResult{NoChoicesOffered: true, Withheld: menu.Withheld}
	}
	if resolutionWithheld != "" {
		// Withholding project_package here removes an entry the customer
		// would otherwise have been offered, unlike the entries
		// AvailableSetupChoices never built: a shorter menu with no
		// explanation reads as "Coach cannot install from my package
		// manager", when the reason is about which directory a single
		// consented command could run in.
		fmt.Fprintf(out, "TypeScript compiler setup: %s is not offered here (%s).\n", SetupChoiceProjectPackage, resolutionWithheld)
	}

	// The diagnosis precedes the menu: a list of installation choices is not
	// an explanation, and a customer asked to authorize a network install
	// deserves to know which check failed and what happens afterward before
	// answering, not after. It names the gap code rather than repeating the
	// caller's own "run --check-project" remediation line, which would be
	// stale advice the moment the install succeeds and the scan resumes.
	fmt.Fprintf(out, "TypeScript compiler setup: this scan cannot resolve a supported TypeScript compiler (%s).\n", gapCode)
	fmt.Fprintln(out, "TypeScript compiler setup: if you confirm a choice below and it succeeds, Coach rechecks readiness and resumes this same scan only if that recheck reports no gap; otherwise no report is produced.")

	reader := bufio.NewReader(in)
	choice, cancelled := promptForCompilerSetupChoice(out, reader, menu.Choices)
	if cancelled {
		return CompilerSetupOfferResult{Cancelled: true}
	}

	if choice == SetupChoiceProjectPackage {
		return runProjectPackageSetupOffer(ctx, dir, revision, configPath, projectPackageDirectory, readiness.Checks.PackageManager, out, reader)
	}
	return runMiseSetupOffer(ctx, dir, revision, configPath, choice, out, reader)
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

// menuOffersExecutableMiseChoice narrows menuOffersExecutableChoice to the
// kinds the interim --prepare-compiler flag can actually run. The scan's own
// offer (RunCompilerSetupOffer) executes every kind and so uses the wider
// predicate; only a printed --prepare-compiler command needs this one.
func menuOffersExecutableMiseChoice(menu SetupChoiceMenu) bool {
	for _, choice := range menu.Choices {
		if choice.Kind == SetupChoiceProjectMise || choice.Kind == SetupChoiceGlobalMise {
			return true
		}
	}
	return false
}

// promptForCompilerSetupChoice requires the user to type one offered
// choice's exact kind name, or "cancel". There is no numbered/default
// selection: an unrecognized or blank answer cancels rather than falling
// back to any choice, mirroring promptForMiseSetupChoice's own contract
// (project_ts_compiler_mise_install.go).
func promptForCompilerSetupChoice(out io.Writer, reader *bufio.Reader, choices []SetupChoice) (SetupChoiceKind, bool) {
	fmt.Fprintln(out, "TypeScript compiler setup: the following choices are offered to resolve the failing compiler check:")
	for _, c := range choices {
		fmt.Fprintf(out, "  - %s%s\n", c.Kind, setupChoiceScopeClause(c.Kind))
	}
	fmt.Fprintln(out, "Type the exact choice name to select it, or 'cancel' to cancel without making any change. There is no default: an unrecognized or blank answer cancels.")
	fmt.Fprint(out, "> ")
	answer, unreadable := readLine(reader)
	if unreadable {
		return "", true
	}
	answer = strings.TrimSpace(answer)
	for _, c := range choices {
		if answer != string(c.Kind) {
			continue
		}
		if c.Kind == SetupChoiceCancel {
			return "", true
		}
		return c.Kind, false
	}
	return "", true
}

// setupChoiceScopeClause names what each choice would touch, at the moment
// the customer picks one. The full AC-SET-2 preview still precedes the
// confirmation, but it arrives only after a selection, so without this the
// selection itself is made from bare machine identifiers -- and
// project_mise and global_mise differ in exactly the property a customer
// would want to know before choosing between them.
func setupChoiceScopeClause(kind SetupChoiceKind) string {
	switch kind {
	case SetupChoiceProjectPackage:
		return " (runs this project's own package manager in the selected manifest context)"
	case SetupChoiceProjectMise:
		return " (installs the version this repository's mise configuration pins, into mise's shared tool store)"
	case SetupChoiceGlobalMise:
		return " (installs the version your global mise configuration pins, into mise's shared tool store)"
	case SetupChoiceCancel:
		return " (change nothing and stop this scan)"
	default:
		return ""
	}
}

// runProjectPackageSetupOffer executes SetupChoiceProjectPackage through its
// own library path (BuildSetupPreview/RunConfirmedSetupAndRecheckReadiness),
// distinct from runMiseSetupOffer's mise install path.
func runProjectPackageSetupOffer(ctx context.Context, dir, revision, configPath, workingDirectory string, packageManager ReadinessCheck, out io.Writer, reader *bufio.Reader) CompilerSetupOfferResult {
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

func printSetupPreview(out io.Writer, preview SetupPreview) {
	fmt.Fprintln(out, "Before this setup command runs, here is exactly what it will do:")
	fmt.Fprintf(out, "  Executable: %s\n", preview.Executable)
	fmt.Fprintf(out, "  Arguments: %s\n", strings.Join(preview.Args, " "))
	fmt.Fprintf(out, "  Working directory: %s\n", preview.WorkingDirectory)
	fmt.Fprintf(out, "  Expected changes: %s\n", preview.ExpectedChanges)
	fmt.Fprintf(out, "  Network use: %s\n", preview.NetworkDisclosure)
	fmt.Fprintf(out, "  Lifecycle-script policy: %s\n", preview.ScriptSuppressionPolicy)
	if preview.PinDisclosure != "" {
		fmt.Fprintf(out, "  Pin disclosure: %s\n", preview.PinDisclosure)
	}
	fmt.Fprintf(out, "  Timeout: %s\n", preview.Timeout)
}

// promptForSetupConfirmation is the single-use explicit confirmation gate
// for a project_package setup command: only the exact token "confirm"
// (case-insensitive) proceeds, read exactly once from reader -- there is no
// retry, mirroring promptForMiseInstallConfirmation's own gate.
func promptForSetupConfirmation(out io.Writer, reader *bufio.Reader) bool {
	fmt.Fprintln(out, "Type 'confirm' to run this setup command now, or anything else to cancel without making any change:")
	fmt.Fprint(out, "> ")
	answer, _ := readLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "confirm")
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

// projectPackageWorkingDirectory resolves BuildSetupPreview's required
// directory the same way checkPackageManager itself does
// (packageManagerContexts): the nearest package.json above a selected policy
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
	worktreeRoot := compilerWorktreeRoot(dir)
	policyPath := configPath
	if policyPath == "" {
		policyPath = defaultProjectConfigPath
	}
	_, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return "", setupChoiceReasonManifestContextUnresolved
	}
	contexts, _ := packageManagerContexts(worktreeRoot, roots)
	switch len(contexts) {
	case 0:
		// Defensive: packageManagerContexts falls back to the worktree root,
		// so it does not return an empty slice today.
		return "", setupChoiceReasonManifestContextUnresolved
	case 1:
		return contexts[0], ""
	default:
		return "", setupChoiceReasonManifestContextAmbiguous
	}
}

// withProjectPackageResolution re-decides the project_package entry against
// the one fact AvailableSetupChoices cannot see: which manifest context the
// install would actually run in. AvailableSetupChoices decides purely from a
// readiness snapshot and has no dir/revision to resolve a working directory
// with, so this is where an offer that cannot be previewed honestly is
// converted into a withheld entry with its reason.
func withProjectPackageResolution(menu SetupChoiceMenu, dir, revision, configPath string) (resolved SetupChoiceMenu, workingDirectory, withheldReason string) {
	if !menuOffersChoice(menu, SetupChoiceProjectPackage) {
		return menu, "", ""
	}
	workingDirectory, withheldReason = projectPackageWorkingDirectory(dir, revision, configPath)
	if withheldReason == "" {
		return menu, workingDirectory, ""
	}
	remaining := make([]SetupChoice, 0, len(menu.Choices))
	for _, choice := range menu.Choices {
		if choice.Kind == SetupChoiceProjectPackage {
			continue
		}
		remaining = append(remaining, choice)
	}
	menu.Choices = remaining
	menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: withheldReason})
	return menu, "", withheldReason
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

// runMiseSetupOffer executes a mise scope choice through its own library
// path (miseScopeDeclaresInstallableCompiler/installMiseTypescriptProject/
// Global, project_ts_compiler_mise_install.go/project_ts_compiler_mise_command.go),
// distinct from runProjectPackageSetupOffer's project-package path. Its
// preview and confirmation prompt are the same ones
// RunPrepareCompilerMiseSetup's standalone --prepare-compiler session uses,
// so a customer sees identical wording regardless of which flow offered the
// same mise scope.
func runMiseSetupOffer(ctx context.Context, dir, revision, configPath string, kind SetupChoiceKind, out io.Writer, reader *bufio.Reader) CompilerSetupOfferResult {
	origin := miseOriginForSetupChoiceKind(kind)
	worktreeRoot := compilerWorktreeRoot(dir)
	version, ok := miseScopeDeclaresInstallableCompiler(origin, worktreeRoot)
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

	if postInstall, err := CheckProjectReadiness(dir, revision, configPath); err == nil {
		result.PostInstallReadiness = postInstall
	}
	result.Succeeded = true
	return result
}

func miseOriginForSetupChoiceKind(kind SetupChoiceKind) string {
	if kind == SetupChoiceGlobalMise {
		return compilerOriginMiseGlobal
	}
	return compilerOriginMiseProject
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
