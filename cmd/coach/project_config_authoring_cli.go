package main

import (
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
	if !codesignalcli.HasControllingTerminal(os.Stdin) {
		fmt.Fprintf(stderr, "%s: no controlling terminal is available; refusing to enter guided policy authoring or write a policy config\n", authorTSUsagePrefix)
		return 2
	}
	return authorProjectConfigTypeScript(dir, f, os.Stdin, stdout, stderr)
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
		fmt.Fprintf(stderr, "%s: TypeScript root discovery did not complete within its budget; the list below may be partial\n", authorTSUsagePrefix)
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
