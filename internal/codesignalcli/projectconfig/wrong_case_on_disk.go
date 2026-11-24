package projectconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// WrongCaseOnDisk is the refusal for a policy path that the working tree under dir does not hold as it is spelled
// while it holds the same path in another case, as .lousy-agents/Coach.json for .lousy-agents/coach.json. named says
// whether the user named the path with --project-config; a path Coach found is checked only at the canonical path,
// the one spelling discovery reads. It is nil when the disk holds the path as spelled, or holds no regular file like
// it.
//
// The disk decides the spelling (ADR-009 decision 5): every component is compared with the directory listing,
// case-folded, so the answer is the same on a disk that folds case and on one that does not, and an entry spelled
// exactly as the path wins over its case variants. What the revision commits does not enter into it. The way out is on
// disk, a rename or the spelling on disk passed instead, and never a commit. An error is a failure to find the
// repository root, never a verdict about the policy.
func WrongCaseOnDisk(run gitrepo.Runner, dir, repoPath string, named bool) (*ConfigError, error) {
	if !named && repoPath != CanonicalPath {
		return nil, nil
	}
	root, err := gitrepo.RepositoryRootWith(run, dir)
	if err != nil {
		return nil, err
	}
	chooser := chosenByCoach
	if named {
		chooser = chosenByTheUser
	}
	return wrongCaseUnder(root, repoPath, chooser, trackedBy(run, root)), nil
}

// trackedBy is whether git's index holds a spelling under root. A question git cannot answer counts as untracked:
// the answer only adds a sentence to the advice.
func trackedBy(run gitrepo.Runner, root string) func(spelling string) bool {
	return func(spelling string) bool {
		output, err := run(root, "ls-files", "-z", "--full-name", "--", ":(top,literal)"+spelling)
		return err == nil && len(output) > 0
	}
}

// wrongCaseUnder is WrongCaseOnDisk for the repository root given.
func wrongCaseUnder(root, repoPath string, chooser pathChooser, tracked func(spelling string) bool) *ConfigError {
	listed, held, truncated, _ := spellingsListed(diskEntries(root), strings.Split(repoPath, "/"))
	if held || slices.ContainsFunc(listed, func(spelling string) bool { return throughARepository(root, spelling) }) {
		return nil
	}
	others := slices.DeleteFunc(listed, func(spelling string) bool { return !regularFileUnder(root, spelling) })
	switch len(others) {
	case 0:
		return nil
	case 1:
		return diskCaseRefusal(fmt.Sprintf("the policy is on disk as %q, but Coach reads %q, so it is not read: %s", others[0], repoPath, oneSpellingOnDisk(others[0], repoPath, chooser)+movedByGit(tracked(others[0]))))
	}
	return diskCaseRefusal(severalSpellingsOnDisk(others, truncated, repoPath, chooser, tracked))
}
