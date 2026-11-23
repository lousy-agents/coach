package projectconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const mergeConflictAdvice = "The index holds an unresolved merge conflict for it; resolve the conflict in it, mark it resolved, and commit it"

// ScanError is the error a scan stops with for a policy on disk that cannot be used, with no other policy read in its
// place. committed is what the committed pipeline said of the same path. Where that is the problem the disk has too,
// the same kind of entry or a committed document that is not valid and is the one on disk, its wording stands, offers
// included: the pipeline has set the replacement to what the file can bear. Otherwise the refusal is worded from the
// disk and offers no replacement, because a pipeline that found the file changed tells the user to commit it, which is
// no advice for a file that cannot be used. unmerged is whether the index holds a conflict for the path, whatever the
// committed copy is: a conflicted file never is the committed document, so the advice to resolve the conflict is added
// to the disk's reason.
func (refusal *DiskPolicyRefusal) ScanError(explicit, unmerged bool, committed error) error {
	var held *ConfigError
	if errors.As(committed, &held) && refusal.sameProblemAs(held) {
		return refusal.CauseWorded(committed)
	}
	subject := fmt.Sprintf("%q", refusal.Path)
	if explicit {
		subject = "--project-config " + subject
	}
	message := fmt.Sprintf("coach codesignal: %s cannot be used on disk (project_config_invalid): %s", subject, refusal.Reason)
	if refusal.Reason == refusalOversize {
		message += fmt.Sprintf(" (the budget is %d bytes)", MaxBytes)
	}
	if cause := refusal.Cause(); cause != "" {
		message += fmt.Sprintf(" (cause=%s)", cause)
	}
	if unmerged {
		message += ". " + mergeConflictAdvice
	}
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: refusal.Cause(), Message: message, UnusableOnDisk: refusal.Cause() == ""}
}

// CauseWorded is err with the refusal's stable cause appended, for a committed pipeline's words that name the entry
// but not the cause the disk has. The cause goes before the final period, so a finished sentence stays finished.
// Words that carry the cause, and a refusal with none, are returned as they are.
func (refusal *DiskPolicyRefusal) CauseWorded(err error) error {
	var held *ConfigError
	cause := refusal.Cause()
	if cause == "" || !errors.As(err, &held) || strings.Contains(held.Message, "cause="+string(cause)) {
		return err
	}
	worded := *held
	body, finished := strings.CutSuffix(worded.Message, ".")
	worded.Message = body + fmt.Sprintf(" (cause=%s)", cause)
	if finished {
		worded.Message += "."
	}
	return &worded
}

// CommittedEntryFits is whether the committed pipeline's words for a policy named with --project-config fit the entry on
// disk. Words that name the entry the revision commits do not fit a different kind of entry at the path; a path that is
// blocked above, or cannot be read, is judged by the pipeline as the disk has it, and every other error is its judgment
// of the file itself.
func (refusal *DiskPolicyRefusal) CommittedEntryFits(err error) bool {
	var held *ConfigError
	if (refusal.Reason != refusalNotRegular && refusal.Reason != refusalLink) || !errors.As(err, &held) {
		return true
	}
	switch held.PolicyCause {
	case projectreadiness.CauseDirectory, projectreadiness.CauseCommittedAsLink:
		return refusal.sameProblemAs(held)
	}
	return true
}

// sameProblemAs is whether the committed pipeline refused the path for the problem the disk has. A committed document
// that is not valid is that problem for a file that is not valid only while the file is that document: once the
// pipeline has found work in it, its words are about the work.
func (refusal *DiskPolicyRefusal) sameProblemAs(held *ConfigError) bool {
	switch refusal.Reason {
	case refusalNotRegular:
		return refusal.Directory && held.PolicyCause == projectreadiness.CauseDirectory
	case refusalLink:
		return held.PolicyCause == projectreadiness.CauseCommittedAsLink
	case refusalInvalid:
		return held.Kind == KindInvalid && held.ReplacesPolicy()
	case refusalOversize:
		return held.Kind == KindUnreadable && held.ReplacesPolicy()
	}
	return false
}

// UnmergedInIndex is whether the index holds an unresolved conflict for repoPath. A question git cannot answer reads as
// no conflict, because the caller already refuses the file and this only decides whether advice is added to the refusal.
func UnmergedInIndex(dir, repoPath string) bool {
	indexed, err := gitrepo.IndexEntries(NewGitRunner(), dir, repoPath)
	return err == nil && holdsUnmergedEntry(withoutEmptyEntries(indexed.Entries))
}
