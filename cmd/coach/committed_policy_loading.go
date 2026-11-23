package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const (
	regularFileMode    = "100644"
	executableFileMode = "100755"
)

// loadCommittedPolicy reads the policy the analyzed revision holds for source. A discovered source names a path
// discovery found committed, and an empty one means nothing is. A named source is committed only if the revision holds
// a blob at its path: a file that is on disk and not committed is the policy on disk alone. Every other named path goes
// through the loader, so that a missing or unusable file, a path that cannot name a policy, and a file inside a
// submodule checkout, which this repository can never commit, keep the refusal they always had.
func loadCommittedPolicy(dir, revision string, source policyref.Ref, disk projectconfig.DiskSelection, ports scanPorts) (committedPolicy, error) {
	if source.Path == "" {
		return committedPolicy{}, nil
	}
	if source.Explicit && projectconfig.ValidatePath(source.Path) == nil {
		held, err := committedAtPath(dir, revision, source.Path)
		if err != nil {
			return committedPolicy{}, err
		}
		if !held && disk.Path != "" && !insideSubmodule(dir, revision, source.Path) {
			return committedPolicy{}, explicitOnDiskRefusal(dir, revision, source.Path)
		}
	}
	content, err := ports.loader(source.JudgedOnDisk())(dir, revision, source.Path)
	if err != nil {
		return committedPolicy{}, err
	}
	return committedPolicy{path: source.Path, content: content}, nil
}

// committedAtPath reports whether the revision holds a regular file at repoPath. A symbolic link is a blob too, but a
// policy cannot be one, so it leaves the path to the working tree exactly as a committed directory or submodule does.
func committedAtPath(dir, revision, repoPath string) (bool, error) {
	entry, found, err := gitrepo.BlobEntryAtRevision(projectconfig.NewGitRunner(), dir, revision, repoPath)
	if err != nil || !found {
		return false, err
	}
	return entry.Mode == regularFileMode || entry.Mode == executableFileMode, nil
}

// explicitOnDiskRefusal is the refusal a named policy has when the working tree cannot hold it, nil for a regular
// file. A file that is not committed is no refusal.
func explicitOnDiskRefusal(dir, revision, repoPath string) error {
	status, err := projectconfig.ExplicitOnDiskStatus(projectconfig.NewGitRunner(), dir, revision, repoPath)
	if err != nil {
		return err
	}
	if status != nil {
		return status
	}
	return nil
}

func insideSubmodule(dir, revision, repoPath string) bool {
	_, cause, _ := projectconfig.UncommittedPolicy(projectconfig.NewGitRunner(), dir, revision, repoPath)
	return cause == projectreadiness.CauseInSubmodule
}
