package codesignalcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
// tree; this mirrors project_snapshot.go's maxSnapshotListBytes/
// maxSnapshotFileBytes finite-input contract so a hostile or enormous tree
// truncates (surfaced as project_config_suggestion_incomplete) instead of
// scanning without bound. 500,000 files comfortably covers even very large
// monorepos while still being finite; MaxInputBytes reuses the existing
// snapshot-listing budget as a generous ceiling on cumulative go.mod/
// go.work content rather than inventing a new constant.
var suggestGoBudgets = projectmodel.GoBudgets{
	MaxInputFiles: 500000,
	MaxInputBytes: maxSnapshotListBytes,
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
// resolution and as the root NewGoSnapshotFS enumerates from, so discovered
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
	revisionSHA, err := ResolveBaselineRevision(dir)
	if err != nil {
		return suggestFailureBeforeDiscovery("", SuggestDiagSnapshotUnavailable, "", snapshotUnavailableMessage("resolve HEAD", err, dir))
	}

	root, err := repositoryRoot(dir)
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

	snapshot, err := NewGoSnapshotFS(root, revisionSHA)
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

// prepareSuggestOutputPath validates --output shape/parent confinement before
// discovery. clean is the repository-relative form used later for the
// create-only write; on a shape rejection clean is "" so the envelope path
// stays repository-relative "when applicable" (issue #220/#210).
func prepareSuggestOutputPath(revisionSHA, root, outputPath string, outputSet bool) (clean string, fail SuggestionResult, ok bool) {
	if !outputSet {
		return "", SuggestionResult{}, true
	}
	clean, valErr := validateOutputPath(root, outputPath)
	if valErr != nil {
		return "", suggestFailureBeforeDiscovery(revisionSHA, SuggestDiagOutputInvalid, clean, valErr.Error()), false
	}
	return clean, SuggestionResult{}, true
}

// writeSuggestCandidate performs the post-discovery create-only --output
// write. An ordinary write failure (read-only directory, out of disk space,
// name too long) is SuggestDiagOutputInvalid; only discovery-result
// serialization maps to SuggestDiagFailed.
func writeSuggestCandidate(root, cleanOutput, outputPath, revisionSHA string, result projectmodel.RootDiscoveryResult, candidate []byte) (fail SuggestionResult, ok bool) {
	exists, writeErr := writeSuggestOutput(root, cleanOutput, candidate)
	if exists {
		return suggestFailureAfterDiscovery(revisionSHA, result, SuggestDiagOutputExists, outputPath, "coach codesignal --suggest-project-config: --output target already exists"), false
	}
	if writeErr != nil {
		return suggestFailureAfterDiscovery(revisionSHA, result, SuggestDiagOutputInvalid, outputPath, writeErr.Error()), false
	}
	return SuggestionResult{}, true
}

func suggestSuccessResult(revisionSHA string, result projectmodel.RootDiscoveryResult, candidate []byte, outputSet bool) SuggestionResult {
	envelope := buildSuggestEnvelope(revisionSHA, result.Roots, result.Coverage, projectmodel.Diagnostic{
		Code:    SuggestDiagReady,
		Message: "coach codesignal --suggest-project-config: candidate generated successfully",
	})

	out := SuggestionResult{Envelope: envelope, ExitCode: 0}
	if !outputSet {
		out.Candidate = candidate
	}
	return out
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

func suggestExitCodeFor(code string) int {
	switch code {
	case SuggestDiagSnapshotUnavailable, SuggestDiagFailed:
		return 3
	default:
		return 2
	}
}

// suggestPrimaryRootDiagnostic maps result's root-discovery diagnostics to
// the single primary suggestion diagnostic per issue #220's fixed priority:
// unavailable > (outside_snapshot|invalid|duplicate|ambiguous) > incomplete
// > no-modules. ok is true only when result represents a usable, complete,
// non-empty root set.
func suggestPrimaryRootDiagnostic(result projectmodel.RootDiscoveryResult) (code, path, message string, ok bool) {
	for _, diag := range result.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagRootUnavailable {
			return SuggestDiagSnapshotUnavailable, diag.Path, suggestDiagnosticMessage(diag), false
		}
	}
	for _, diag := range result.Coverage.Diagnostics {
		switch diag.Code {
		case projectmodel.DiagRootOutsideSnapshot, projectmodel.DiagRootInvalid, projectmodel.DiagRootDuplicate, projectmodel.DiagRootAmbiguous:
			return SuggestDiagAmbiguousRoots, diag.Path, suggestDiagnosticMessage(diag), false
		}
	}
	if !result.Complete {
		for _, diag := range result.Coverage.Diagnostics {
			if diag.Code == projectmodel.DiagRootIncomplete {
				return SuggestDiagIncomplete, diag.Path, suggestDiagnosticMessage(diag), false
			}
		}
		return SuggestDiagIncomplete, "", "coach codesignal --suggest-project-config: Go root discovery did not complete within its resource budget", false
	}
	if len(result.Roots) == 0 {
		return SuggestDiagNoGoModules, "", "coach codesignal --suggest-project-config: no Go module or workspace root was found at HEAD", false
	}
	return "", "", "", true
}

// snapshotUnavailableMessage builds the diagnostic message for a
// SuggestDiagSnapshotUnavailable failure -- resolving HEAD, resolving the
// repository root, or opening the HEAD snapshot filesystem -- without ever
// letting an absolute host filesystem path reach the NDJSON envelope.
//
// underlyingErr's text routinely embeds an absolute path. Three error
// shapes are handled structurally, by extracting the failure reason from
// data the error already carries, in priority order:
//
//  1. *fs.PathError (filepath.EvalSymlinks inside repositoryRoot): unwrapped
//     to just its errno-class Err, discarding Path entirely, since the path
//     there was never known to the caller and cannot be stripped by
//     substring match.
//  2. *OperationalError (resolveHEAD's "not inside a Git worktree" case):
//     its Reason() carries the same failure with no path interpolated.
//  3. *snapshotListError (NewGoSnapshotFS's ls-tree listing failure): its
//     Unwrap() carries the underlying git failure alone, with dir dropped
//     from the wrapping fmt.Errorf -- but git's own stderr text can still
//     embed dir itself, so this case is not a guarantee, only a narrowing;
//     see the scrub loop below.
//
// The structural extraction above narrows the surface a path could hide in,
// but does not guarantee it: *snapshotListError.Unwrap() (and any other
// error shape, e.g. repositoryRoot's own bare git-plumbing-failure text)
// still carries raw git stderr, which can itself embed an absolute path
// (e.g. `fatal: cannot change to '<dir>': No such file or directory`) that
// no structural extraction step removes. So every path reaching this point
// -- from every case, not just the unstructured fallback -- still has each
// absolute path the caller does know about (its invocation directory and,
// once resolved, the repository root) replaced with "." if present, both in
// its raw form and in the %q-quoted form (via strconv.Quote, which escapes
// '"', '\', and control bytes -- and on Windows always differs from the raw
// form because of '\'-separated paths). This is a no-op for the
// *fs.PathError and *OperationalError cases, whose extracted reason never
// contains a path in the first place.
func snapshotUnavailableMessage(op string, underlyingErr error, knownAbsolutePaths ...string) string {
	reason := underlyingErr.Error()

	var pathErr *fs.PathError
	var opErr *OperationalError
	var listErr *snapshotListError
	switch {
	case errors.As(underlyingErr, &pathErr):
		reason = pathErr.Err.Error()
	case errors.As(underlyingErr, &opErr):
		reason = opErr.Reason()
	case errors.As(underlyingErr, &listErr):
		reason = listErr.Unwrap().Error()
	}

	for _, absolutePath := range knownAbsolutePaths {
		if absolutePath == "" {
			continue
		}
		reason = strings.ReplaceAll(reason, absolutePath, ".")
		if quoted := strconv.Quote(absolutePath); len(quoted) >= 2 {
			reason = strings.ReplaceAll(reason, quoted[1:len(quoted)-1], ".")
		}
	}

	return fmt.Sprintf("coach codesignal --suggest-project-config: could not %s: %s", op, reason)
}

func suggestDiagnosticMessage(diag projectmodel.Diagnostic) string {
	if diag.Message != "" {
		return diag.Message
	}
	return diag.Code
}

type suggestionCandidate struct {
	SchemaVersion string   `json:"schema_version"`
	Roots         []string `json:"roots"`
}

// serializeSuggestionCandidate renders roots as the strict schema-1
// project-config candidate: 2-space indent, one trailing newline, fixed
// key order, sorted and deduplicated roots.
func serializeSuggestionCandidate(roots []string) ([]byte, error) {
	candidate := suggestionCandidate{
		SchemaVersion: "1",
		Roots:         normalizeSuggestionRoots(roots),
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// normalizeSuggestionRoots defensively re-sorts and deduplicates
// DiscoverGoRoots' already-sorted, deduplicated Roots so the candidate
// contract holds even if that upstream invariant is ever relaxed.
func normalizeSuggestionRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	out = append(out, roots...)
	sort.Strings(out)
	deduped := out[:0]
	var previous string
	for i, root := range out {
		if i == 0 || root != previous {
			deduped = append(deduped, root)
			previous = root
		}
	}
	return deduped
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

// suggestCoverageWireFrom renders in as the envelope's stderr wire shape:
// see suggestCoverageWire's doc comment for the two fields it normalizes.
// It never mutates in, including a caller-owned DiscoverGoRoots result.
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

// validateSuggestOutputPathShape rejects an --output value that can never
// be a valid create-only target: empty, the literal "-", absolute, not
// normalized, escaping the repository, or containing a ".git" component.
// It returns the cleaned repository-relative path on success.
func validateSuggestOutputPathShape(outputPath string) (string, error) {
	if outputPath == "" {
		return "", fmt.Errorf("must be a non-empty repository-relative path")
	}
	if outputPath == "-" {
		return "", fmt.Errorf("must not be \"-\"")
	}
	if filepath.IsAbs(outputPath) {
		return "", fmt.Errorf("must be relative to the repository root, not an absolute path")
	}
	clean := filepath.Clean(outputPath)
	if clean != outputPath || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("must be a normalized path that stays inside the repository")
	}
	for _, segment := range strings.Split(filepath.ToSlash(clean), "/") {
		if strings.EqualFold(segment, ".git") {
			return "", fmt.Errorf("must not contain a \".git\" path component")
		}
	}
	return clean, nil
}

// checkOutputParents walks every existing parent component of cleanOutput
// (relative to repositoryRootDir) via os.Lstat -- not os.Stat -- so a
// symlinked parent directory is rejected rather than silently followed. A
// missing parent is also rejected: a create-only write must never mkdir -p.
func checkOutputParents(repositoryRootDir, cleanOutput string) error {
	segments := strings.Split(filepath.ToSlash(cleanOutput), "/")
	current := repositoryRootDir
	for i := 0; i < len(segments)-1; i++ {
		current = filepath.Join(current, segments[i])
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("parent directory %q does not exist", strings.Join(segments[:i+1], "/"))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("parent path component %q is a symlink", strings.Join(segments[:i+1], "/"))
		}
		if !info.IsDir() {
			return fmt.Errorf("parent path component %q is not a directory", strings.Join(segments[:i+1], "/"))
		}
	}
	return nil
}

// validateOutputPath performs the shape and parent-confinement stages of
// --output validation only. Whether the target already exists is checked
// separately, after discovery succeeds (see writeSuggestOutput's O_EXCL
// create-only open): issue #220 places "an existing --output target" as
// the last failure-precedence stage, after root discovery, not before it.
//
// clean is returned even on a parent-confinement error, since a valid
// repository-relative form is already known once shape validation passes;
// only a shape-validation failure -- where no valid relative form exists
// yet -- returns clean == "". Callers use this to avoid putting an
// absolute or otherwise un-cleaned caller-supplied path into a
// diagnostic's path field.
func validateOutputPath(repositoryRootDir, outputPath string) (clean string, err error) {
	clean, shapeErr := validateSuggestOutputPathShape(outputPath)
	if shapeErr != nil {
		return "", shapeErr
	}
	if parentErr := checkOutputParents(repositoryRootDir, clean); parentErr != nil {
		return clean, parentErr
	}
	return clean, nil
}

// unwrapPathError rebuilds err as "<cleanOutput>: <errno>" when it is a
// *fs.PathError, dropping the absolute host filesystem path the error
// otherwise carries; every other diagnostic message in this feature is
// repository-relative; err is returned unchanged if it is not a
// *fs.PathError.
func unwrapPathError(cleanOutput string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("%s: %w", cleanOutput, pathErr.Err)
	}
	return err
}

// writeSuggestOutput performs the authoritative create-only write: an
// O_EXCL open that fails with fs.ErrExist when the target already exists.
// This is the only existence check for --output -- there is deliberately no
// preflight stat, both because issue #220 puts "target already exists" last
// in the failure precedence (after root discovery) and because O_EXCL leaves
// no TOCTOU window: a concurrently created target is never clobbered, and a
// symlink at the target position is never followed.
//
// A failed Write or Close (ENOSPC, EIO, ...) leaves target removed: the
// O_EXCL create already succeeded, so without this cleanup a truncated file
// would remain at target and make the next run fail with
// SuggestDiagOutputExists instead of surfacing the original write failure
// again. Removal is best-effort -- its own error is never propagated, and
// the original writeErr/closeErr is always what is returned.
func writeSuggestOutput(repositoryRootDir, cleanOutput string, candidate []byte) (exists bool, err error) {
	target := filepath.Join(repositoryRootDir, filepath.FromSlash(cleanOutput))
	file, openErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if openErr != nil {
		if errors.Is(openErr, fs.ErrExist) {
			return true, nil
		}
		return false, unwrapPathError(cleanOutput, openErr)
	}
	_, writeErr := file.Write(candidate)
	closeErr := file.Close()
	if writeErr != nil {
		os.Remove(target)
		return false, unwrapPathError(cleanOutput, writeErr)
	}
	if closeErr != nil {
		os.Remove(target)
		return false, unwrapPathError(cleanOutput, closeErr)
	}
	return false, nil
}
