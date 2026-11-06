package projectconfig

import (
	"fmt"
	"path"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// WrongCaseAtRevision is the refusal for the policy Coach discovers when nothing is committed at the path it
// reads and the revision holds that path in another case, as .Lousy-Agents/Coach.json for
// .lousy-agents/coach.json. The policy is committed, so offering to author one is wrong, and falling back
// to no policy at all would drop its layer checks without a word. It is nil when the revision holds
// nothing like the path, or cannot be asked, which leaves the policy plainly missing.
//
// It is for a policy Coach found. A path the user named is told in its own words, since the user typed it.
func WrongCaseAtRevision(run gitrepo.Runner, dir, revision, repoPath string) *ConfigError {
	return wrongCaseOf(entriesThrough(run, dir, revision), repoPath)
}

func wrongCaseOf(entries revisionEntries, repoPath string) *ConfigError {
	spelled, ambiguous, changed := spellingInRevision(entries, strings.Split(repoPath, "/"))
	if ambiguous != nil {
		return wrongCaseRefusal(fmt.Sprintf("the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: keep one of them at that path and remove the others", quotedList(ambiguous), repoPath))
	}
	if !changed {
		return nil
	}
	held := strings.Join(spelled, "/")
	return wrongCaseRefusal(fmt.Sprintf("the policy is committed as %q, but Coach reads %q, so it is not read: %s", held, repoPath, moveAdvice(held, repoPath)))
}

func wrongCaseRefusal(reason string) *ConfigError {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseNameCaseDiffers, Message: "coach codesignal: " + reason + " (project_config_invalid)"}
}

// moveAdvice is how to move the committed file to the path Coach reads. The directory is made first because
// on a file system that tells cases apart it does not exist under the new spelling, and git mv does not
// create it; where the file system folds case, mkdir -p finds the old one and changes nothing.
func moveAdvice(from, to string) string {
	command, ok := gitMoveCommand(from, to)
	if !ok {
		return "move it to that path and commit the rename"
	}
	if directory := path.Dir(to); directory != "." {
		command = "mkdir -p " + shellword.Quote(directory) + " && " + command
	}
	return "rename it with " + command + " and commit the rename"
}
