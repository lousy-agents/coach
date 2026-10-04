package codesignalcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/lousy-agents/coach/pkg/projectmodel"

	"sort"
)

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
func suggestExitCodeFor(code string) int {
	switch code {
	case SuggestDiagSnapshotUnavailable, SuggestDiagFailed:
		return 3
	default:
		return 2
	}
}
func suggestDiagnosticMessage(diag projectmodel.Diagnostic) string {
	if diag.Message != "" {
		return diag.Message
	}
	return diag.Code
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
