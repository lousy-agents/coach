package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/configauthoring"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/revisionfs"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// discoverTSRoots is a seam over projectmodel.DiscoverTSRoots, following the
// same override-for-testing pattern as loadProjectConfig/resolveProjectBackend
// in project_analysis.go: it lets tests drive authorProjectConfigTypeScript's
// DiagTSRootUnavailable-vs-DiagTSRootIncomplete branch directly, since a real
// Git snapshot (revisionfs.New) can never itself produce
// DiagTSRootUnavailable -- its root directory always opens successfully.
var discoverTSRoots = projectmodel.DiscoverTSRoots

const authorTSUsagePrefix = "coach codesignal --baseline --suggest-project-config --project-language typescript"

// tsAuthoringRootBudgets bounds the DiscoverTSRoots walk the guided
// TypeScript authoring dispatch runs over the immutable baseline snapshot,
// mirroring the finite-budget contract configauthoring/suggest.go's
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

// authorProjectConfigTypeScript performs runAuthorProjectConfigTypeScript's
// work after the controlling-terminal gate: resolving the baseline
// revision/repository root/snapshot, discovering TypeScript roots, and
// running the guided authoring session. It takes stdin explicitly, rather
// than reading os.Stdin directly, for the same reason codesignalcli.
// AuthorProjectConfig itself takes an io.Reader instead of a terminal: it
// makes this function callable directly in a test with a controlling-
// terminal *os.File standing in for the caller's stdin, without a real pty.
// terminal.HasControllingTerminal's own contract deliberately forbids
// faking its true result, so the gate stays in runAuthorProjectConfigTypeScript
// and is not itself exercised this way -- only the logic downstream of it.
func authorProjectConfigTypeScript(dir string, f codesignalFlags, stdin, stdout, stderr *os.File) int {
	revision, err := gitrepo.ResolveBaselineRevision(dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not resolve the baseline revision: %s\n", authorTSUsagePrefix, err)
		return 3
	}

	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, err)
		return 3
	}

	if err := rejectUnusableAuthoringOutput(root, f); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", authorTSUsagePrefix, err)
		return 2
	}

	snapshot, err := revisionfs.New(root, revision)
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

	result := configauthoring.Author(root, stdin, stderr, stdout, discovered, f.output, f.outputSet)
	return reportAuthoringResult(result, stderr)
}
