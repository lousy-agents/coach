package projectconfig

import (
	"fmt"
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// spellingToKeep is the spelling the policy should have on disk: the path Coach reads, except for the default policy's
// path named in another case, which every scan without --project-config reads by its default spelling.
func spellingToKeep(repoPath string, chooser pathChooser) string {
	if namesTheDefaultPolicyInAnotherCase(repoPath, chooser) {
		return CanonicalPath
	}
	return repoPath
}

// oneSpellingOnDisk is the way out for a policy the disk lists under one other spelling. For the default policy's path
// named in another case the rename is the wrong way: every scan that passes no flag reads the default spelling.
func oneSpellingOnDisk(onDisk, typed string, chooser pathChooser) string {
	if namesTheDefaultPolicyInAnotherCase(typed, chooser) && onDisk == CanonicalPath {
		return fmt.Sprintf("pass the spelling on disk, %q, or leave --project-config off, which reads that spelling by itself; to rename the file on disk to %q would make every scan without the flag refuse it", onDisk, typed)
	}
	target := spellingToKeep(typed, chooser)
	return fmt.Sprintf("rename the file on disk to %q, or pass the spelling on disk, %q%s.%s", target, onDisk, withTheFlag(chooser), directoryInCase(onDisk, target))
}

// withTheFlag says how a policy Coach found can be read at another spelling: the user names it.
func withTheFlag(chooser pathChooser) string {
	if chooser == chosenByTheUser {
		return ""
	}
	return ", with --project-config"
}

// severalSpellingsOnDisk names every spelling the disk lists besides the one Coach reads, so that none is read unseen,
// and says how to keep one. With more spellings than are named, none is chosen.
func severalSpellingsOnDisk(others []string, truncated bool, repoPath string, chooser pathChooser, tracked func(spelling string) bool) string {
	listed := slices.Sorted(slices.Values(others))
	named := quotedList(listed)
	if truncated {
		named += ", and more"
	}
	reason := fmt.Sprintf("the disk lists the policy under several spellings, %s, and Coach reads %q: rename the file on disk or pass the spelling on disk of the one to read%s, and remove the others", named, repoPath, withTheFlag(chooser))
	if truncated {
		return reason + "."
	}
	target := spellingToKeep(repoPath, chooser)
	keep := keptSpelling(others, target)
	rest := slices.DeleteFunc(slices.Clone(listed), func(spelling string) bool { return spelling == keep })
	if keep == target {
		return fmt.Sprintf("%s. To keep %q, remove %s.", reason, keep, quotedList(rest))
	}
	return fmt.Sprintf("%s. To keep %q, rename it on disk to %q and remove %s.%s%s", reason, keep, target, quotedList(rest), directoryInCase(keep, target), movedByGit(tracked(keep)))
}

// movedByGit is the advice for a policy git tracks: on a disk that folds case, a rename on disk alone leaves the index
// with the old spelling, which every other checkout then reads.
func movedByGit(tracked bool) string {
	if !tracked {
		return ""
	}
	return " Rename it with git's own move, so that the index records the rename as well as the disk."
}

func diskCaseRefusal(reason string) *ConfigError {
	return &ConfigError{
		Kind:        KindInvalid,
		Remedy:      projectreadiness.RemedyNone,
		PolicyCause: projectreadiness.CauseNameCaseDiffers,
		Message:     fmt.Sprintf("coach codesignal: %s (project_config_invalid) (cause=%s)", reason, projectreadiness.CauseNameCaseDiffers),
	}
}
