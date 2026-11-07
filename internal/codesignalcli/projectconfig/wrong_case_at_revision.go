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
	return wrongCaseOf(entriesThrough(run, dir, revision), repoPath, moveGuardFor(run, dir))
}

func wrongCaseOf(entries revisionEntries, repoPath string, mayMove moveGuard) error {
	spellings, truncated, err := spellingsInRevision(entries, strings.Split(repoPath, "/"))
	if err != nil {
		return unansweredListing(fmt.Sprintf("nothing is committed at %q, the path Coach reads for the policy", repoPath), err)
	}
	spellings = slices.DeleteFunc(spellings, func(spelling string) bool { return spelling == repoPath })
	switch len(spellings) {
	case 0:
		return nil
	case 1:
		return wrongCaseRefusal(fmt.Sprintf("the policy is committed as %q, but Coach reads %q, so it is not read: %s", spellings[0], repoPath, moveAdvice(spellings[0], repoPath, mayMove)))
	}
	return wrongCaseRefusal(severalSpellingsReason(spellings, truncated, repoPath, mayMove))
}

func wrongCaseRefusal(reason string) *ConfigError {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseNameCaseDiffers, Message: "coach codesignal: " + reason + " (project_config_invalid)"}
}

// moveAdvice is how to move the committed file to the path Coach reads. The directory is made first because
// on a file system that tells cases apart it does not exist under the new spelling, and git mv does not
// create it; where the file system folds case, mkdir -p finds the old one and changes nothing. Both run at
// the repository root, found once: the variable keeps a failure to find it from becoming a mkdir at the
// root of the file system, because the && stops the chain.
func moveAdvice(from, to string, mayMove moveGuard) string {
	command, ok := moveCommand(from, to)
	if !ok {
		return "move it to that path and commit the rename"
	}
	if !mayMove(from, to) {
		return fmt.Sprintf("a different file stands at %q, and a move could overwrite it, so no command is printed: move that file aside, then move the committed one to that path and commit the rename", to)
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
