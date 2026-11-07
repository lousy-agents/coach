package projectconfig

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// WrongCaseAtRevision is the refusal for the policy Coach discovers when nothing is committed at the path it
// reads and the revision holds that path in another case, as .Lousy-Agents/Coach.json for
// .lousy-agents/coach.json. The policy is committed, so offering to author one is wrong, and falling back
// to no policy at all would drop its layer checks without a word. It is nil when the revision holds
// nothing like the path. A revision that cannot be listed is a refusal too: the policy is not plainly missing
// while git has not said whether it is committed in another case.
//
// The refusal is a *ConfigError. It is returned as an error so that nil is the answer "nothing like it".
//
// It is for a policy Coach found. A path the user named is told in its own words, since the user typed it.
func WrongCaseAtRevision(run gitrepo.Runner, dir, revision, repoPath string) error {
	return wrongCaseOf(entriesThrough(run, dir, revision), repoPath)
}

func wrongCaseOf(entries revisionEntries, repoPath string) error {
	spellings, truncated, err := spellingsInRevision(entries, strings.Split(repoPath, "/"))
	if err != nil {
		return unansweredListing(fmt.Sprintf("nothing is committed at %q, the path Coach reads for the policy", repoPath), err)
	}
	spellings = slices.DeleteFunc(spellings, func(spelling string) bool { return spelling == repoPath })
	switch len(spellings) {
	case 0:
		return nil
	case 1:
		return wrongCaseRefusal(fmt.Sprintf("the policy is committed as %q, but Coach reads %q, so it is not read: %s", spellings[0], repoPath, moveAdvice(spellings[0], repoPath)))
	}
	return wrongCaseRefusal(severalSpellingsReason(spellings, truncated, repoPath))
}

// severalSpellingsReason names every spelling the revision holds, so that none stays committed unseen, and
// prints the commands that keep one of them: it is moved to the path Coach reads, and the others leave the
// index. They leave the index only because where a file system folds case they name the file just moved. With
// more spellings than are named, or one that cannot be shown to a terminal, no command is printed: it would
// leave a spelling committed or print a name it cannot show.
func severalSpellingsReason(spellings []string, truncated bool, repoPath string) string {
	listed := slices.Sorted(slices.Values(spellings))
	named := quotedList(listed)
	if truncated {
		named += ", and more"
	}
	reason := fmt.Sprintf("the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: keep one of them at that path and remove the others", named, repoPath)
	keep := keptSpelling(spellings, repoPath)
	move, movable := moveCommand(keep, repoPath)
	others := slices.DeleteFunc(slices.Clone(listed), func(spelling string) bool { return spelling == keep })
	if truncated || !movable {
		return reason
	}
	quoted := make([]string, len(others))
	for i, other := range others {
		quoted[i] = shellword.Quote(other)
	}
	return fmt.Sprintf("%s; to keep %q, run %s && git -C \"$top\" --literal-pathspecs rm --cached -q -- %s and commit", reason, keep, move, strings.Join(quoted, " "))
}

func wrongCaseRefusal(reason string) *ConfigError {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseNameCaseDiffers, Message: "coach codesignal: " + reason + " (project_config_invalid)"}
}

// moveAdvice is how to move the committed file to the path Coach reads. The directory is made first because
// on a file system that tells cases apart it does not exist under the new spelling, and git mv does not
// create it; where the file system folds case, mkdir -p finds the old one and changes nothing. Both run at
// the repository root, found once: the variable keeps a failure to find it from becoming a mkdir at the
// root of the file system, because the && stops the chain.
func moveAdvice(from, to string) string {
	command, ok := moveCommand(from, to)
	if !ok {
		return "move it to that path and commit the rename"
	}
	return "rename it with " + command + " and commit the rename"
}

// moveCommand is the commands that move from to to at the repository root, and false for a name that cannot
// be shown to a terminal as it is.
func moveCommand(from, to string) (string, bool) {
	if !movable(from, to) {
		return "", false
	}
	move := `git -C "$top" mv -- ` + shellword.Quote(from) + " " + shellword.Quote(to)
	if directory := path.Dir(to); directory != "." {
		move = `mkdir -p "$top"/` + shellword.Quote(directory) + " && " + move
	}
	return `top="$(git rev-parse --show-toplevel)" && ` + move, true
}
