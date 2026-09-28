package codesignalcli

import (
	"errors"
	"fmt"

	"sort"
	"strings"
)

// hasDuplicateOrOverlappingPaths reports exact duplicates or ancestor/descendant
// path pairs. Used for layer prefixes, which must partition policy membership.
// Complexity is O(n log n) via sort + adjacent/ancestor checks rather than a
// nested all-pairs scan.
func hasDuplicateOrOverlappingPaths(paths []string) bool {
	if len(paths) < 2 {
		return false
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	for i := 0; i < len(sorted); i++ {
		if sorted[i] == "." {

			if len(sorted) > 1 {
				return true
			}
			continue
		}
		if i+1 < len(sorted) {
			left, right := sorted[i], sorted[i+1]
			if left == right || strings.HasPrefix(right, left+"/") {
				return true
			}
		}
	}
	return false
}

// loadProjectConfigForReadiness reads and validates repoPath at revision
// like LoadProjectConfig, but keeps a git-read failure distinct from a
// content/schema rejection instead of collapsing both into
// *ProjectConfigError: checkPolicy must report the former as an
// *OperationalError (exit 1, fail closed) and only the latter as the
// policy_invalid gap. It returns the decoded projectConfig directly so
// checkPolicy needs no second decode of the same bytes.
//
// A stdout-size-budget failure is deliberately classified as content
// rejection (policy_invalid, exit 0) rather than operational (exit 1),
// diverging from LoadProjectConfig's projectConfigGitError, which reports
// every *gitOperationalBoundError -- including this same size-budget case --
// as project_config_invalid at exit 2. The committed policy file's size is
// something its author controls and can fix, unlike a corrupt object store
// or a timed-out git process, so --check-project's read-only, actionable-gap
// contract treats it as a gap rather than an environment failure. A timeout
// or stderr-budget failure still reports *OperationalError: those indicate a
// resource/environment condition, not a defect in the file's content.
func loadProjectConfigForReadiness(dir, revision, repoPath string) (projectConfig, error) {
	if err := validateProjectConfigPath(repoPath); err != nil {
		return projectConfig{}, projectConfigError(repoPath, revision, err.Error())
	}

	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		var boundErr *gitOperationalBoundError
		if errors.As(err, &boundErr) && boundErr.kind == gitOperationalBoundStdout {
			return projectConfig{}, projectConfigError(repoPath, revision, fmt.Sprintf("committed content exceeds the %d-byte size budget: %s", maxProjectConfigBytes, err))
		}
		return projectConfig{}, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: --project-config %q could not be read at revision %q: %s", repoPath, revision, err)}
	}

	config, err := parseProjectConfig(data)
	if err != nil {
		return projectConfig{}, projectConfigError(repoPath, revision, err.Error())
	}
	return config, nil
}
