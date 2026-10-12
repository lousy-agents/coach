package configauthoring

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/revisionfs"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// suggestGoBudgets bounds the DiscoverGoRoots walk --suggest-project-config
// runs over the immutable HEAD snapshot. A zero-value GoBudgets means
// unbounded, which is unsafe for a CLI reading a repository-controlled
// tree; this mirrors revisionfs.MaxListBytes/
// revisionfs per-file finite-input contract so a hostile or enormous tree
// truncates (surfaced as project_config_suggestion_incomplete) instead of
// scanning without bound. 500,000 files comfortably covers even very large
// monorepos while still being finite; MaxInputBytes reuses the existing
// snapshot-listing budget as a generous ceiling on cumulative go.mod/
// go.work content rather than inventing a new constant.
var suggestGoBudgets = projectmodel.GoBudgets{
	MaxInputFiles: 500000,
	MaxInputBytes: revisionfs.MaxListBytes,
}

// SuggestionResult is the outcome of one Suggest call.
// Envelope is always populated (the stderr diagnostic/provenance
// document); Candidate is populated only on success when no output path
// was requested (the caller writes it to stdout). ExitCode follows the
// exit-code table in issue #220: 0 on success, 2 for a usage/discovery
// rejection, 3 for a snapshot or serialization failure.
type SuggestionResult struct {
	Candidate []byte
	Envelope  []byte
	ExitCode  int
}

// Suggest resolves HEAD, discovers Go module/workspace roots
// over an immutable Git snapshot via pkg/projectmodel.DiscoverGoRoots, and
// produces a minimal schema-1 project-config candidate. It never mutates
// the repository worktree and never writes to outputPath unless
// outputSet is true, in which case the write is create-only (it fails if
// outputPath already exists in any form).
//
// dir may be any directory inside the Git worktree, not necessarily its
// root (issue #220 must support invocation from a subdirectory): the
// repository root is resolved once and used both for --output path
// resolution and as the root revisionfs.New enumerates from, so discovered
// roots and --output are always repository-root-relative regardless of the
// invocation directory.
//
// Every failure, including a HEAD-resolution failure (not a Git worktree,
// no commits yet) and a repository-root resolution failure, is folded into
// the returned SuggestionResult rather than a separate error: issue #220
// states every invocation (except --help) writes exactly one envelope, and
// its diagnostic table treats an unresolvable HEAD or repository root as
// the degenerate case of "resolved immutable snapshot cannot be read"
// (SuggestDiagSnapshotUnavailable).
func Suggest(dir, outputPath string, outputSet bool) SuggestionResult {
	revisionSHA, err := gitrepo.ResolveBaselineRevision(dir)
	if err != nil {
		return suggestFailureBeforeDiscovery("", SuggestDiagSnapshotUnavailable, "", snapshotUnavailableMessage("resolve HEAD", err, dir))
	}

	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return suggestFailureBeforeDiscovery(revisionSHA, SuggestDiagSnapshotUnavailable, "", snapshotUnavailableMessage("resolve the repository root", err, dir))
	}

	// The output-path shape/confinement check runs here, before discovery,
	// per issue #220's failure precedence; whether the target already
	// exists is checked only after discovery succeeds (see writeSuggestCandidate)
	// so that, e.g., a repository with no Go modules at all still reports
	// SuggestDiagNoGoModules rather than SuggestDiagOutputExists for an
	// unrelated pre-existing --output target.
	cleanOutput, prepFail, ok := prepareSuggestOutputPath(revisionSHA, root, outputPath, outputSet)
	if !ok {
		return prepFail
	}

	snapshot, err := revisionfs.New(root, revisionSHA)
	if err != nil {
		return suggestFailureBeforeDiscovery(revisionSHA, SuggestDiagSnapshotUnavailable, "", snapshotUnavailableMessage("read the HEAD snapshot", err, root, dir))
	}

	result, discoverErr := projectmodel.DiscoverGoRoots(snapshot, suggestGoBudgets)
	if discoverErr != nil {
		// DiscoverGoRoots' documented contract is "never returns a non-nil
		// error"; this guards against that contract changing silently.
		return suggestFailureAfterDiscovery(revisionSHA, result, SuggestDiagFailed, "", discoverErr.Error())
	}

	if code, path, message, ok := suggestPrimaryRootDiagnostic(result); !ok {
		return suggestFailureAfterDiscovery(revisionSHA, result, code, path, message)
	}

	candidate, err := serializeSuggestionCandidate(result.Roots)
	if err != nil {
		return suggestFailureAfterDiscovery(revisionSHA, result, SuggestDiagFailed, "", err.Error())
	}

	if outputSet {
		if writeFail, writeOK := writeSuggestCandidate(root, cleanOutput, outputPath, revisionSHA, result, candidate); !writeOK {
			return writeFail
		}
	}

	return suggestSuccessResult(revisionSHA, result, candidate, outputSet)
}

// InvalidArgumentsSuggestionEnvelope builds the stderr diagnostic/
// provenance envelope for a --suggest-project-config invocation rejected
// before any Git or discovery work runs (issue #220's failure precedence
// step 1: flag/mode validation). Revision is always "" and roots_considered
// is always [] at this stage.
func InvalidArgumentsSuggestionEnvelope(message string) []byte {
	return buildSuggestEnvelope("", nil, zeroSuggestCoverage(), projectmodel.Diagnostic{
		Code:    SuggestDiagInvalidArguments,
		Message: message,
	})
}

// Suggestion diagnostic codes for `coach codesignal --baseline
// --suggest-project-config` (issue #220). Exactly one of these appears as
// the primary diagnostic in every invocation's stderr envelope.
const (
	SuggestDiagInvalidArguments    = "project_config_suggestion_invalid_arguments"
	SuggestDiagOutputInvalid       = "project_config_suggestion_output_invalid"
	SuggestDiagOutputExists        = "project_config_suggestion_output_exists"
	SuggestDiagNoGoModules         = "project_config_suggestion_no_go_modules"
	SuggestDiagAmbiguousRoots      = "project_config_suggestion_ambiguous_roots"
	SuggestDiagIncomplete          = "project_config_suggestion_incomplete"
	SuggestDiagSnapshotUnavailable = "project_config_suggestion_snapshot_unavailable"
	SuggestDiagFailed              = "project_config_suggestion_failed"
	SuggestDiagReady               = "project_config_suggestion_ready"
)
