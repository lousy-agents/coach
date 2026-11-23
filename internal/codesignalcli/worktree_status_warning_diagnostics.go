package codesignalcli

import (
	"strconv"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

var (
	unreadableDirectoryReason = hiddenReason{
		name:  "unreadable_directory",
		cause: "git could not open a directory, so the worktree scan did not read what is under it (make it readable to include it)",
	}
)

func unrecognizedWarningReason(count int) hiddenReason {
	noun := " warnings"
	if count == 1 {
		noun = " warning"
	}
	return hiddenReason{
		name:  "unrecognized_git_warning",
		cause: "git printed " + strconv.Itoa(count) + noun + " while reading the working tree that coach does not recognize, so part of the tree may not have been read",
	}
}

// statusWarningDiagnostics reports what git warned about while reading the
// status. A directory it could not open is omitted from the status without
// failing, so the files under it were never candidates, and a warning the scan
// does not recognize could be about anything it did not read. Neither says what
// was unread, so both are unknown and qualify the verdict. git's own words are
// never relayed.
func statusWarningDiagnostics(warnings gitrepo.StatusWarnings) []codesignal.Diagnostic {
	var diagnostics []codesignal.Diagnostic
	if len(warnings.UnreadableDirectories) > 0 {
		verdicts := make(map[string]string, len(warnings.UnreadableDirectories))
		for _, directory := range warnings.UnreadableDirectories {
			verdicts[directory] = codesignal.HidesInScopeUnknown
		}
		diagnostics = reasonDiagnostics(unreadableDirectoryReason, verdicts)
	}
	if warnings.Unrecognized > 0 {
		diagnostics = append(diagnostics, unrecognizedWarningReason(warnings.Unrecognized).entry("", codesignal.HidesInScopeUnknown))
	}
	return diagnostics
}
