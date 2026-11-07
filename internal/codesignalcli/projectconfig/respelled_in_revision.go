package projectconfig

import (
	"fmt"
	"path"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// respelledInRevision is the refusal for a policy that is not on disk when the
// analyzed revision holds its path in another case. The disk has no spelling to
// compare, so the revision's own listing says how the path is spelled. It is
// nil when the revision holds the path as typed, holds no spelling of it, or
// cannot be asked. A listing git could not give is no answer, and the refusal says so instead of leaving the
// policy plainly missing, which would offer to author one over a file that may be committed.
func respelledInRevision(entries revisionEntries, repoPath string) *ConfigError {
	spelled, ambiguous, changed, err := spellingInRevision(entries, strings.Split(repoPath, "/"))
	if err != nil {
		return unansweredListing(fmt.Sprintf("--project-config %q is not on disk", repoPath), err)
	}
	if ambiguous != nil {
		return newUnusableOnDisk(repoPath, projectreadiness.CauseNameCaseDiffers, fmt.Sprintf("is not on disk, and the analyzed revision holds more than one spelling of it: %s; pass the one you mean", quotedList(ambiguous)))
	}
	if !changed {
		return nil
	}
	corrected := strings.Join(spelled, "/")
	return newUnusableOnDisk(repoPath, projectreadiness.CauseNameCaseDiffers, fmt.Sprintf("is not on disk, and the analyzed revision holds it as %q%s", corrected, passClause("; pass", corrected)))
}

// spellingInRevision resolves each component against the revision's listing of
// the directory above it: as typed when the revision has it, as the only case
// variant when it has exactly one, and ambiguous, as full paths, when it has
// several. A component the revision holds in no spelling ends the walk with
// nothing found. A listing git could not give is the error, never nothing found.
func spellingInRevision(entries revisionEntries, parts []string) (spelled, ambiguous []string, changed bool, err error) {
	directory := ""
	for _, part := range parts {
		names, err := entries(directory)
		if err != nil {
			return nil, nil, false, err
		}
		name, variants := resolveComponent(names, part)
		if variants != nil {
			return nil, joinedBelow(directory, variants), false, nil
		}
		if name == "" {
			return nil, nil, false, nil
		}
		changed = changed || name != part
		spelled = append(spelled, name)
		directory = path.Join(directory, name)
	}
	return spelled, nil, changed, nil
}
