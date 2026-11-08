package projectconfig

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// WrongCaseAtRevision is the refusal for a policy path that the analyzed revision does not commit as it is spelled
// while it commits the same path in another case, as .Lousy-Agents/Coach.json for .lousy-agents/coach.json. The
// policy is committed, so offering to author one is wrong, and falling back to no policy at all would drop its layer
// checks without a word. It is nil when the revision commits the path as spelled, or commits nothing like it.
//
// What the revision commits is the only thing that decides it: not the spelling a disk shows, not whether git ignores
// case, and not what the index holds. A policy Coach finds, one the user names, and the project check are told the
// same on any disk, and the advice is in words: a rename that is already staged is told what one that is not staged
// is, and says to commit it.
//
// The refusal is a *ConfigError. A revision that cannot be listed is a refusal too, which carries the failure: the
// policy is not plainly missing while git has not said whether it is committed in another case.
func WrongCaseAtRevision(run gitrepo.Runner, dir, revision, repoPath string) error {
	return wrongCaseOf(entriesThrough(run, dir, revision), repoPath, chosenByCoach)
}

// wrongCaseOfNamedPolicy is WrongCaseAtRevision for a path the user named with --project-config, which has one more way
// out: passing the spelling the revision commits.
func wrongCaseOfNamedPolicy(run gitrepo.Runner, dir, revision, repoPath string) error {
	return wrongCaseOf(entriesThrough(run, dir, revision), repoPath, chosenByTheUser)
}

func wrongCaseOf(entries revisionEntries, repoPath string, chooser pathChooser) error {
	others, held, truncated, err := spellingsInRevision(entries, strings.Split(repoPath, "/"))
	if err != nil {
		return unansweredListing(fmt.Sprintf("nothing is committed at %q, the path Coach reads for the policy", repoPath), err)
	}
	if held || len(others) == 0 {
		return nil
	}
	if keepsTheDefaultSpelling(repoPath, others, chooser) {
		return wrongCaseRefusal(keepTheDefaultPolicy(others, truncated, repoPath))
	}
	record := whereToRecord(repoPath, chooser)
	if len(others) == 1 {
		return wrongCaseRefusal(fmt.Sprintf("the policy is committed as %q, but Coach reads %q, so it is not read: %s", others[0], repoPath, recordTheRename(others[0], repoPath, record, chooser)))
	}
	return wrongCaseRefusal(severalSpellingsReason(others, truncated, repoPath, record, chooser))
}

func wrongCaseRefusal(reason string) *ConfigError {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseNameCaseDiffers, Message: "coach codesignal: " + reason + " (project_config_invalid)"}
}
