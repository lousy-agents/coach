package projectconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
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
	return wrongCaseUnder(root, repoPath, chooser), nil
}

// wrongCaseUnder is WrongCaseOnDisk for the repository root given.
func wrongCaseUnder(root, repoPath string, chooser pathChooser) *ConfigError {
	listed, held, truncated, _ := spellingsListed(diskEntries(root), strings.Split(repoPath, "/"))
	if held || slices.ContainsFunc(listed, func(spelling string) bool { return throughARepository(root, spelling) }) {
		return nil
	}
	others := slices.DeleteFunc(listed, func(spelling string) bool { return !regularFileUnder(root, spelling) })
	switch len(others) {
	case 0:
		return nil
	case 1:
		return diskCaseRefusal(fmt.Sprintf("the policy is on disk as %q, but Coach reads %q, so it is not read: %s", others[0], repoPath, oneSpellingOnDisk(others[0], repoPath, chooser)))
	}
	return diskCaseRefusal(severalSpellingsOnDisk(others, truncated, repoPath, chooser))
}

// regularFileUnder is whether a spelling names a regular file under root. Anything else there, a directory, a link or
// a pipe, is no policy under another name.
func regularFileUnder(root, spelling string) bool {
	opened, err := os.OpenRoot(root)
	if err != nil {
		return false
	}
	defer func() { _ = opened.Close() }()
	info, err := opened.Lstat(filepath.FromSlash(spelling))
	return err == nil && info.Mode().IsRegular()
}

// spellingToKeep is the spelling the policy should have on disk: the path Coach reads, except for the default policy's
// path named in another case, which every scan without --project-config reads by its default spelling.
func spellingToKeep(repoPath string, chooser pathChooser) string {
	if namesTheDefaultPolicyInAnotherCase(repoPath, chooser) {
		return CanonicalPath
	}
	return repoPath
}

// throughARepository is whether a spelling runs through a directory, below the root, that holds a .git entry: the
// checkout of a submodule or another repository. A policy there cannot be committed here, which the submodule refusal
// says; a difference in case below it is not the reason.
func throughARepository(root, spelling string) bool {
	parts := strings.Split(spelling, "/")
	for depth := 1; depth < len(parts); depth++ {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(strings.Join(parts[:depth], "/")), ".git")); err == nil {
			return true
		}
	}
	return false
}

// diskEntries lists a directory of the working tree, through the repository root so that a link cannot lead the walk
// outside it. A directory that cannot be listed has no spellings to offer: the checks of the path itself say why it
// cannot be read.
func diskEntries(root string) directoryEntries {
	return func(directory string) ([]string, error) {
		opened, err := os.OpenRoot(root)
		if err != nil {
			return nil, nil
		}
		defer func() { _ = opened.Close() }()
		name := "."
		if directory != "" {
			name = filepath.FromSlash(directory)
		}
		listing, err := opened.OpenFile(name, os.O_RDONLY|openWithoutWaiting, 0)
		if err != nil {
			return nil, nil
		}
		if info, err := listing.Stat(); err != nil || !info.IsDir() {
			_ = listing.Close()
			return nil, nil
		}
		defer func() { _ = listing.Close() }()
		names, err := listing.Readdirnames(-1)
		if err != nil {
			return nil, nil
		}
		return names, nil
	}
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
func severalSpellingsOnDisk(others []string, truncated bool, repoPath string, chooser pathChooser) string {
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
	return fmt.Sprintf("%s. To keep %q, rename it on disk to %q and remove %s.%s", reason, keep, target, quotedList(rest), directoryInCase(keep, target))
}

func diskCaseRefusal(reason string) *ConfigError {
	return &ConfigError{
		Kind:        KindInvalid,
		Remedy:      projectreadiness.RemedyNone,
		PolicyCause: projectreadiness.CauseNameCaseDiffers,
		Message:     fmt.Sprintf("coach codesignal: %s (project_config_invalid) (cause=%s)", reason, projectreadiness.CauseNameCaseDiffers),
	}
}
