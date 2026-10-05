package configauthoring

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/revisionfs"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func suggestFailureBeforeDiscovery(revision, code, path, message string) SuggestionResult {
	envelope := buildSuggestEnvelope(revision, nil, zeroSuggestCoverage(), projectmodel.Diagnostic{Code: code, Path: path, Message: message})
	return SuggestionResult{Envelope: envelope, ExitCode: suggestExitCodeFor(code)}
}

func suggestFailureAfterDiscovery(revision string, result projectmodel.RootDiscoveryResult, code, path, message string) SuggestionResult {
	envelope := buildSuggestEnvelope(revision, result.Roots, result.Coverage, projectmodel.Diagnostic{Code: code, Path: path, Message: message})
	return SuggestionResult{Envelope: envelope, ExitCode: suggestExitCodeFor(code)}
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
//  1. *fs.PathError (filepath.EvalSymlinks inside gitrepo.RepositoryRoot): unwrapped
//     to just its errno-class Err, discarding Path entirely, since the path
//     there was never known to the caller and cannot be stripped by
//     substring match.
//  2. *gitrepo.OperationalError (gitrepo.resolveHEAD's "not inside a Git worktree" case):
//     its Reason() carries the same failure with no path interpolated.
//  3. *revisionfs.ListError (revisionfs.New's ls-tree listing failure): its
//     Unwrap() carries the underlying git failure alone, with dir dropped
//     from the wrapping fmt.Errorf -- but git's own stderr text can still
//     embed dir itself, so this case is not a guarantee, only a narrowing;
//     see the scrub loop below.
//
// The structural extraction above narrows the surface a path could hide in,
// but does not guarantee it: *revisionfs.ListError.Unwrap() (and any other
// error shape, e.g. gitrepo.RepositoryRoot's own bare git-plumbing-failure text)
// still carries raw git stderr, which can itself embed an absolute path
// (e.g. `fatal: cannot change to '<dir>': No such file or directory`) that
// no structural extraction step removes. So every path reaching this point
// -- from every case, not just the unstructured fallback -- still has each
// absolute path the caller does know about (its invocation directory and,
// once resolved, the repository root) replaced with "." if present, both in
// its raw form and in the %q-quoted form (via strconv.Quote, which escapes
// '"', '\', and control bytes -- and on Windows always differs from the raw
// form because of '\'-separated paths). This is a no-op for the
// *fs.PathError and *gitrepo.OperationalError cases, whose extracted reason never
// contains a path in the first place.
func snapshotUnavailableMessage(op string, underlyingErr error, knownAbsolutePaths ...string) string {
	reason := underlyingErr.Error()

	var pathErr *fs.PathError
	var opErr *gitrepo.OperationalError
	var listErr *revisionfs.ListError
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

func suggestExitCodeFor(code string) int {
	switch code {
	case SuggestDiagSnapshotUnavailable, SuggestDiagFailed:
		return 3
	default:
		return 2
	}
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
