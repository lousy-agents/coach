package configauthoring

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultSuggestConfigRel is the repository-root-relative destination when
// --suggest-project-config is invoked without --output.
const DefaultSuggestConfigRel = ".lousy-agents/coach.json"

const defaultSuggestConfigDir = ".lousy-agents"

// EnsureDefaultSuggestConfigDir creates the single .lousy-agents directory
// at the repository root when it is missing. It is the one mkdir a
// create-only suggestion write may perform, and only for that implicit
// default destination: checkOutputParents still rejects a missing parent of
// an explicit --output and never creates one. A symlink at .lousy-agents is
// rejected and not followed.
func EnsureDefaultSuggestConfigDir(repositoryRootDir string) error {
	dir := filepath.Join(repositoryRootDir, defaultSuggestConfigDir)
	info, err := os.Lstat(dir)
	switch {
	case err != nil && os.IsNotExist(err):
		return unwrapPathError(DefaultSuggestConfigRel, os.Mkdir(dir, 0o755))
	case err != nil:
		return unwrapPathError(defaultSuggestConfigDir, err)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("parent path component %q is a symlink", defaultSuggestConfigDir)
	case !info.IsDir():
		return fmt.Errorf("parent path component %q is not a directory", defaultSuggestConfigDir)
	default:
		return nil
	}
}

func ValidateAuthoringOutputPath(repositoryRootDir, outputPath string) (clean string, err error) {
	return validateOutputPath(repositoryRootDir, outputPath)
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
