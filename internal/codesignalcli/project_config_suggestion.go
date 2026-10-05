package codesignalcli

import (
	"encoding/json"
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/revisionfs"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

// suggestGoBudgets bounds the DiscoverGoRoots walk --suggest-project-config
// runs over the immutable HEAD snapshot. A zero-value GoBudgets means
// unbounded, which is unsafe for a CLI reading a repository-controlled
// tree; this mirrors project_snapshot.go's revisionfs.MaxListBytes/
// revisionfs.maxSnapshotFileBytes finite-input contract so a hostile or enormous tree
// truncates (surfaced as project_config_suggestion_incomplete) instead of
// scanning without bound. 500,000 files comfortably covers even very large
// monorepos while still being finite; MaxInputBytes reuses the existing
// snapshot-listing budget as a generous ceiling on cumulative go.mod/
// go.work content rather than inventing a new constant.
var suggestGoBudgets = projectmodel.GoBudgets{
	MaxInputFiles: 500000,
	MaxInputBytes: revisionfs.MaxListBytes,
}

// SuggestionResult is the outcome of one SuggestProjectConfig call.
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

// SuggestProjectConfig resolves HEAD, discovers Go module/workspace roots
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
func SuggestProjectConfig(dir, outputPath string, outputSet bool) SuggestionResult {
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

func suggestFailureBeforeDiscovery(revision, code, path, message string) SuggestionResult {
	envelope := buildSuggestEnvelope(revision, nil, zeroSuggestCoverage(), projectmodel.Diagnostic{Code: code, Path: path, Message: message})
	return SuggestionResult{Envelope: envelope, ExitCode: suggestExitCodeFor(code)}
}

func suggestFailureAfterDiscovery(revision string, result projectmodel.RootDiscoveryResult, code, path, message string) SuggestionResult {
	envelope := buildSuggestEnvelope(revision, result.Roots, result.Coverage, projectmodel.Diagnostic{Code: code, Path: path, Message: message})
	return SuggestionResult{Envelope: envelope, ExitCode: suggestExitCodeFor(code)}
}

type suggestionCandidate struct {
	SchemaVersion string   `json:"schema_version"`
	Roots         []string `json:"roots"`
}

type suggestionEnvelope struct {
	DiagnosticVersion string                    `json:"diagnostic_version"`
	Kind              string                    `json:"kind"`
	Revision          string                    `json:"revision"`
	HeuristicVersion  string                    `json:"heuristic_version"`
	RootsConsidered   []string                  `json:"roots_considered"`
	Coverage          suggestCoverageWire       `json:"coverage"`
	Diagnostics       []projectmodel.Diagnostic `json:"diagnostics"`
}

// suggestCoverageWire mirrors projectmodel.Coverage for this envelope's
// stderr wire shape, with two deliberate deviations from projectmodel.
// Coverage's own JSON tags: Phase is always the mandated
// "project_config_suggestion" (issue #220), never DiscoverGoRoots' own
// "go_root_discovery" phase name, which only describes pkg/projectmodel's
// internal call; and Diagnostics has no "omitempty", so an empty slice
// marshals as the literal "[]" issue #220 (and #210, whose coverage shape
// this reuses) require, rather than being omitted entirely.
type suggestCoverageWire struct {
	Phase       string                    `json:"phase"`
	Complete    bool                      `json:"complete"`
	Counts      map[string]int            `json:"counts,omitempty"`
	Budgets     map[string]int            `json:"budgets,omitempty"`
	Diagnostics []projectmodel.Diagnostic `json:"diagnostics"`
}

func suggestCoverageWireFrom(in projectmodel.Coverage) suggestCoverageWire {
	diagnostics := in.Diagnostics
	if diagnostics == nil {
		diagnostics = []projectmodel.Diagnostic{}
	}
	return suggestCoverageWire{
		Phase:       "project_config_suggestion",
		Complete:    in.Complete,
		Counts:      in.Counts,
		Budgets:     in.Budgets,
		Diagnostics: diagnostics,
	}
}

// buildSuggestEnvelope renders the single stderr provenance/diagnostic
// document every --suggest-project-config invocation writes (except
// --help): one UTF-8 newline-delimited JSON (NDJSON) object -- compact,
// single-line, no indentation -- followed by exactly one trailing newline,
// with fixed key order. This differs deliberately from the stdout
// candidate, which is 2-space-indented multi-line JSON meant for a human to
// read and commit.
func buildSuggestEnvelope(revision string, roots []string, coverage projectmodel.Coverage, primary projectmodel.Diagnostic) []byte {
	rootsConsidered := make([]string, len(roots))
	copy(rootsConsidered, roots)
	sort.Strings(rootsConsidered)

	envelope := suggestionEnvelope{
		DiagnosticVersion: "1",
		Kind:              "project_config_suggestion",
		Revision:          revision,
		HeuristicVersion:  "go-project-config-roots@1",
		RootsConsidered:   rootsConsidered,
		Coverage:          suggestCoverageWireFrom(coverage),
		Diagnostics:       []projectmodel.Diagnostic{primary},
	}
	// envelope's fields are plain strings/bools/maps[string]int/slices of
	// small structs, so Marshal cannot fail for this shape.
	data, _ := json.Marshal(envelope)
	return append(data, '\n')
}

// zeroSuggestCoverage synthesizes the Coverage shape reported before any
// DiscoverGoRoots result exists (invalid-arguments, output-path validation,
// and snapshot-open failures), matching the counts/budgets vocabulary
// DiscoverGoRoots itself reports.
func zeroSuggestCoverage() projectmodel.Coverage {
	return projectmodel.Coverage{
		// Phase is overwritten by buildSuggestEnvelope's suggestCoverageWireFrom
		// regardless of what is set here; left as "project_config_suggestion"
		// for readability at call sites that inspect this value directly.
		Phase:    "project_config_suggestion",
		Complete: false,
		Counts: map[string]int{
			"files_seen":      0,
			"files_skipped":   0,
			"modules_seen":    0,
			"modules_skipped": 0,
			"roots_emitted":   0,
		},
		Budgets: projectmodel.EffectiveGoBudgets(suggestGoBudgets),
	}
}

func ValidateAuthoringOutputPath(repositoryRootDir, outputPath string) (clean string, err error) {
	return validateOutputPath(repositoryRootDir, outputPath)
}
