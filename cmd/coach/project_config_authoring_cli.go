package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// discoverTSRoots is a seam over projectmodel.DiscoverTSRoots, following the
// same override-for-testing pattern as loadProjectConfig/resolveProjectBackend
// in main.go: it lets tests drive authorProjectConfigTypeScript's
// DiagTSRootUnavailable-vs-DiagTSRootIncomplete branch directly, since a real
// Git snapshot (codesignalcli.NewGoSnapshotFS) can never itself produce
// DiagTSRootUnavailable -- its root directory always opens successfully.
var discoverTSRoots = projectmodel.DiscoverTSRoots

const authorTSUsagePrefix = "coach codesignal --baseline --suggest-project-config --project-language typescript"

// tsAuthoringRootBudgets bounds the DiscoverTSRoots walk the guided
// TypeScript authoring dispatch runs over the immutable baseline snapshot,
// mirroring the finite-budget contract project_config_suggestion.go's
// suggestGoBudgets already applies to the equivalent Go root-discovery walk.
var tsAuthoringRootBudgets = projectmodel.GoBudgets{
	MaxInputFiles: 500000,
	MaxInputBytes: 64 << 20,
}

// runAuthorProjectConfigTypeScript dispatches `coach codesignal --baseline
// --suggest-project-config --project-language typescript`. The
// controlling-terminal check runs before any revision resolution, snapshot
// read, or discovery: without a controlling terminal on stdin, this function
// never prompts and never writes a policy file.
//
// Exit codes deliberately match the plain `--suggest-project-config` family's
// documented table (SuggestionResult/suggestExitCodeFor): 0 success, 2
// usage/discovery rejection (no controlling terminal, or a declined/
// cancelled/invalid guided-authoring outcome -- this dispatch's own
// interactive-decision equivalent of a discovery rejection), 3 for a failure
// resolving or reading the immutable revision/repository-root/snapshot this
// dispatch discovers TypeScript roots over, or for root discovery itself
// failing outright (as opposed to merely reporting an incomplete walk).
// What deliberately does NOT match: the report shape. This dispatch is
// interactive (it prompts over a real terminal), so its stderr is plain,
// human-facing text rather than the machine-readable NDJSON envelope
// `--suggest-project-config` writes, and an absolute invocation-directory
// path is acceptable in that text where it would not be in the envelope.
func runAuthorProjectConfigTypeScript(dir string, f codesignalFlags, stdout, stderr *os.File) int {
	if reason := interactiveRefusalReason(f, os.Stdin); reason != "" {
		fmt.Fprintf(stderr, "%s: %s; refusing to enter guided policy authoring or write a policy config. Draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>.\n", authorTSUsagePrefix, reason)
		return 2
	}
	return authorProjectConfigTypeScript(dir, f, os.Stdin, stdout, stderr)
}

// scanShouldAuthorProjectConfig reports whether a real scan's (not
// --check-project/--suggest-project-config/--prepare-compiler) analysis
// error is AC-POL-8's guided-authoring case: a TypeScript policy that was
// never committed at all (*codesignalcli.ProjectConfigError with Kind ==
// ProjectConfigNotFound, whether or not it has been wrapped in a
// *ProjectConfigErrorWithReadiness for AC-SET-13) with a controlling
// terminal available on stdin. An unusable configPath is rejected before
// even inspecting err, since ValidateProjectConfigPath's own failure is a
// usage error, not an absent policy. Every other *ProjectConfigError kind
// (a usage error that reached classifyAnalysisError some other way, a file
// that exists uncommitted in the worktree, or a transient git operational
// failure), a CompilerUnresolvedError (--prepare-compiler's compiler-setup
// flow owns that gap instead), any other error, a non-TypeScript
// --project-language (the guided session below is TypeScript-specific), or
// no controlling terminal, or noInteractive being true (R1's escape hatch:
// --no-interactive or a non-empty CI environment variable, see
// nonInteractiveRequested in main.go) all fall through to
// classifyAnalysisError's existing message-only path unchanged.
func scanShouldAuthorProjectConfig(err error, language, configPath string, noInteractive bool) bool {
	if language != "typescript" {
		return false
	}
	if codesignalcli.ValidateProjectConfigPath(configPath) != nil {
		return false
	}
	var configErr *codesignalcli.ProjectConfigError
	if !errors.As(err, &configErr) {
		return false
	}
	if configErr.Kind != codesignalcli.ProjectConfigNotFound {
		return false
	}
	if noInteractive {
		return false
	}
	return codesignalcli.HasControllingTerminal(os.Stdin)
}

// runScanProjectConfigAuthoring implements AC-POL-8. It reuses the exact
// guided authoring session `--suggest-project-config --project-language
// typescript` runs (authorProjectConfigTypeScript) rather than re-deriving
// authoring logic, but overrides that command's own success exit code: a
// successful, approved session there returns 0 because the standalone
// command's job is done once the candidate is written, whereas here the
// same candidate has not been reviewed or committed yet, so this
// invocation must still refuse to analyze it -- exit 2, no CodeSignal
// report (none is ever rendered on this path), and an instruction to
// review, commit, and rerun.
//
// A scan can never set --output (validateCodesignalFlags rejects it outside
// --suggest-project-config), so the approved candidate is emitted to stdout
// rather than written to disk -- the suggestion-mode shape the epic freezes
// for an omitted --output, and AC-EVD-8's "policy-candidate document" case.
// Nothing is created on disk here, which is why the closing instruction
// names stdout and the --project-config path the customer must save it to:
// telling them a candidate "was created" would name an artifact they cannot
// find, review, or commit.
//
// scanErr is the *ProjectConfigError (possibly wrapped in a
// *ProjectConfigErrorWithReadiness) that scanShouldAuthorProjectConfig just
// matched. It is printed before the interactive session opens so AC-SET-13's
// "report all gaps" clause holds on this controlling-terminal branch too, not
// only on classifyAnalysisError's no-controlling-terminal path: without it, a
// simultaneously failing compiler check would stay masked until the customer
// approved, committed, and reran, discovering the compiler gap only then.
func runScanProjectConfigAuthoring(dir string, f codesignalFlags, scanErr error, stdout, stderr *os.File) int {
	printProjectConfigGapBeforeAuthoring(scanErr, stderr)

	exitCode := authorProjectConfigTypeScript(dir, f, os.Stdin, stdout, stderr)
	if exitCode != 0 {
		return exitCode
	}
	fmt.Fprintf(stderr, "coach codesignal: the approved TypeScript project-config candidate was written to stdout and no file was created; save it to %q, review it, commit it, then rerun this scan once it is committed.\n", f.projectConfig)
	return 2
}

// printProjectConfigGapBeforeAuthoring prints the same two lines
// classifyAnalysisError would have printed for scanErr's class-2 config
// failure -- its own message, plus AC-SET-13's masked-gap lines when scanErr
// was wrapped with a readiness snapshot -- before guided authoring takes over
// stderr with its own prompts.
func printProjectConfigGapBeforeAuthoring(scanErr error, stderr *os.File) {
	var configErr *codesignalcli.ProjectConfigError
	if !errors.As(scanErr, &configErr) {
		return
	}
	fmt.Fprintln(stderr, configErr.Message)

	var withReadiness *codesignalcli.ProjectConfigErrorWithReadiness
	if errors.As(scanErr, &withReadiness) {
		for _, line := range codesignalcli.AlsoFailingGapLines(withReadiness.Readiness, withReadiness.ConfigPath) {
			fmt.Fprintln(stderr, line)
		}
	}
}

// rejectUnusableAuthoringOutput fails fast on --output shape or an existing
// target before the session. writeSuggestOutput's O_EXCL remains the sole
// existence authority against a concurrently created target.
func rejectUnusableAuthoringOutput(root string, f codesignalFlags) error {
	if !f.outputSet {
		return nil
	}
	clean, err := codesignalcli.ValidateAuthoringOutputPath(root, f.output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, clean)); err == nil {
		return fmt.Errorf("--output target already exists")
	}
	return nil
}

// authorProjectConfigTypeScript performs runAuthorProjectConfigTypeScript's
// work after the controlling-terminal gate: resolving the baseline
// revision/repository root/snapshot, discovering TypeScript roots, and
// running the guided authoring session. It takes stdin explicitly, rather
// than reading os.Stdin directly, for the same reason codesignalcli.
// AuthorProjectConfig itself takes an io.Reader instead of a terminal: it
// makes this function callable directly in a test with a controlling-
// terminal *os.File standing in for the caller's stdin, without a real pty.
// codesignalcli.HasControllingTerminal's own contract deliberately forbids
// faking its true result, so the gate stays in runAuthorProjectConfigTypeScript
// and is not itself exercised this way -- only the logic downstream of it.
func authorProjectConfigTypeScript(dir string, f codesignalFlags, stdin, stdout, stderr *os.File) int {
	revision, err := codesignalcli.ResolveBaselineRevision(dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not resolve the baseline revision: %s\n", authorTSUsagePrefix, err)
		return 3
	}

	root, err := codesignalcli.AuthoringRepositoryRoot(dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, err)
		return 3
	}

	if err := rejectUnusableAuthoringOutput(root, f); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, err)
		return 2
	}

	snapshot, err := codesignalcli.NewGoSnapshotFS(root, revision)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not read the baseline snapshot: %s\n", authorTSUsagePrefix, err)
		return 3
	}

	discovered, err := discoverTSRoots(snapshot, tsAuthoringRootBudgets)
	if err != nil {
		// DiscoverTSRoots' documented contract is "never returns a non-nil
		// error"; this guards against that contract changing silently, the
		// same way SuggestProjectConfig treats an equivalent DiscoverGoRoots
		// guard as SuggestDiagFailed (exit 3), not a usage/discovery (exit 2)
		// rejection.
		fmt.Fprintf(stderr, "%s: TypeScript root discovery failed: %s\n", authorTSUsagePrefix, err)
		return 3
	}
	if diag, unavailable := tsRootDiscoverySnapshotUnavailable(discovered); unavailable {
		fmt.Fprintf(stderr, "%s: could not read the TypeScript root-discovery snapshot (%s); refusing to enter guided authoring against a partial or empty root list\n", authorTSUsagePrefix, diag.Path)
		return 3
	}
	if !discovered.Complete {
		// A budget-truncated walk must never be presented to the customer
		// as an authoritative root list: unlike an unavailable snapshot
		// (exit 3, hard failure), the partial data collected so far is
		// real, but incomplete input is still not something an approval
		// gate can safely be built on top of. This mirrors the batch
		// --suggest-project-config path, which maps the equivalent
		// DiscoverGoRoots truncation to SuggestDiagIncomplete (exit 2) and
		// refuses to write, rather than warning and proceeding.
		fmt.Fprintf(stderr, "%s: TypeScript root discovery did not complete within its budget; refusing to enter guided authoring against a partial root list\n", authorTSUsagePrefix)
		return 2
	}

	result := codesignalcli.AuthorProjectConfig(root, stdin, stderr, stdout, discovered, f.output, f.outputSet)
	return reportAuthoringResult(result, stderr)
}

// tsRootDiscoverySnapshotUnavailable reports discovered's DiagTSRootUnavailable
// diagnostic, if present. DiscoverTSRoots' Complete field goes false for two
// distinct causes, and callers must not conflate them: DiagTSRootUnavailable
// means a read failure -- either the whole walk (Path ".", Roots/Candidates
// come back completely empty) or a single tsconfig.json/package.json the
// walk otherwise continued past (Path is that file, and Roots/Candidates may
// already hold real entries collected before the failure) -- while
// DiagTSRootIncomplete means mere budget truncation, where the partial list
// gathered so far is real data. Either DiagTSRootUnavailable case is treated
// as a hard failure here, matching the Go discovery family's own
// DiagRootUnavailable handling (any occurrence, whole-walk or single-file,
// maps to SuggestDiagSnapshotUnavailable): a read failure means some fact
// about the tree could not be established, so the roots collected around it
// are not trusted as a complete picture either. Only DiagTSRootIncomplete is
// safe to warn about and still show to the user.
func tsRootDiscoverySnapshotUnavailable(discovered projectmodel.TSRootDiscoveryResult) (projectmodel.Diagnostic, bool) {
	for _, diag := range discovered.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagTSRootUnavailable {
			return diag, true
		}
	}
	return projectmodel.Diagnostic{}, false
}

// reportAuthoringResult translates one AuthorProjectConfig session outcome
// into the process's exit code. A declined/cancelled session and every
// failure mode share exit 2, the same usage/discovery-failure exit code
// --suggest-project-config already uses; only Approved with no validation,
// existing-target, or write error is success. The approved candidate itself
// has already reached stdout or disk inside AuthorProjectConfig -- there is
// nothing left to write here.
func reportAuthoringResult(result codesignalcli.AuthoringResult, stderr *os.File) int {
	if !result.Approved {
		fmt.Fprintf(stderr, "%s: authoring was cancelled or not approved; no policy config was written\n", authorTSUsagePrefix)
		return 2
	}
	if result.ValidationError != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, result.ValidationError)
		return 2
	}
	if result.OutputExists {
		fmt.Fprintf(stderr, "%s: --output target already exists\n", authorTSUsagePrefix)
		return 2
	}
	if result.WriteError != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, result.WriteError)
		return 2
	}
	return 0
}
