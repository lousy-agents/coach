package projectconfig

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// canonicalOnDisk is what the working tree holds at the default policy's path, asked the way a policy named
// with --project-config is asked, so that the scan and --check-project say the same of a path that is not
// committed. A regular file the process can read is a policy that is not committed; nothing there is no
// diagnostic; and anything else (a directory, a link, a file where its directory should be, a path the process
// cannot search or read) is a path that cannot be a policy, and says why. The result carries only the fields of
// Selection that describe the disk.
func canonicalOnDisk(root string) (Selection, error) {
	return canonicalOnDiskThrough(checkRegularFileUnder, root)
}

// canonicalOnDiskThrough is canonicalOnDisk with the probe of the path given, so that a probe that fails in a way
// that is not a verdict about the path (it returns no refusal) can be told apart from every verdict.
func canonicalOnDiskThrough(probe func(root, repoPath string) error, root string) (Selection, error) {
	return selectionOfTheProbe(probe(root, CanonicalPath))
}

func selectionOfTheProbe(err error) (Selection, error) {
	if err == nil {
		return Selection{UncommittedCanonical: true}, nil
	}
	var mismatch caseMismatchError
	if errors.As(err, &mismatch) {
		return selectionOfASpellingOnDisk(mismatch)
	}
	var refusal *ConfigError
	switch {
	case !errors.As(err, &refusal):
		return Selection{}, err
	case refusal.AbsentFromDisk():
		return Selection{}, nil
	}
	return Selection{UnusableCanonical: refusal}, nil
}

// selectionOfASpellingOnDisk is the selection for a default policy path that a disk folding case keeps in another
// spelling. It is asked only when the revision commits nothing at the path in any spelling, so there is no policy
// for the spelling to be wrong about, and it never stops the scan: the explicit path's refusal says to pass or to
// rename a spelling the revision holds, and the user passed no path. What stands under the other spelling is a
// file that cannot be the policy, named in a diagnostic; when the policy is not there at all, the directory is
// another tool's, or empty, and is nothing.
func selectionOfASpellingOnDisk(mismatch caseMismatchError) (Selection, error) {
	if mismatch.displaced != nil {
		return selectionOfTheProbe(mismatch.displaced)
	}
	return Selection{UnusableCanonical: mismatch.config(nil)}, nil
}

// onDiskReasons say, for each cause the disk can give a path that cannot be the policy, what is wrong with it.
var onDiskReasons = map[projectreadiness.PolicyCause]string{
	projectreadiness.CauseNotARegularFile:    "it is not a regular file",
	projectreadiness.CauseSymbolicLink:       "it is a symbolic link, or is reached through one",
	projectreadiness.CauseUnreadableOnDisk:   "the process cannot check or read it",
	projectreadiness.CauseParentNotDirectory: "a file stands where its directory should be",
	projectreadiness.CauseNameCaseDiffers:    "it is spelled in another case on disk",
}

// UnusableCanonicalMessage is the scan's diagnostic for a default policy path the working tree holds something
// at that cannot be the policy. It names the cause --check-project reports for the same path, in the form the
// text output of that command prints it, so that a reader or a program matching either finds the same word.
func UnusableCanonicalMessage(refusal *ConfigError) string {
	reason, known := onDiskReasons[refusal.PolicyCause]
	if refusal.PolicyCause == projectreadiness.CauseInSubmodule {
		reason = fmt.Sprintf("it lies inside the submodule %q, which this repository records as a single commit, so the file cannot be committed here", refusal.Blocker)
	} else if refusal.PolicyCause == projectreadiness.CauseNameCaseDiffers && refusal.SpelledOnDisk != "" {
		reason = fmt.Sprintf("the working tree has %q, which is spelled in another case", refusal.SpelledOnDisk)
	} else if !known {
		reason = "the working tree cannot use it"
	}
	return fmt.Sprintf("%s is in the worktree but cannot be the policy (cause=%s): %s, so this scan did not read it.", CanonicalPath, refusal.PolicyCause, reason)
}
