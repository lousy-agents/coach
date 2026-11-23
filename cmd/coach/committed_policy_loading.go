package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// loadCommittedPolicy reads the policy the analyzed revision holds for source, refusing one that is not usable. A
// discovered source names a path discovery found committed, and an empty one means nothing is. It is the policy a scan
// uses when the working tree selects none, so a missing or unusable file, a path that cannot name a policy, and a file
// inside a submodule checkout keep the refusal they always had.
func loadCommittedPolicy(dir, revision string, source policyref.Ref, ports scanPorts) (committedPolicy, error) {
	if source.Path == "" {
		return committedPolicy{}, nil
	}
	content, err := ports.loader(source.JudgedOnDisk())(dir, revision, source.Path)
	if err != nil {
		return committedPolicy{}, err
	}
	return committedPolicy{path: source.Path, content: content}, nil
}

// readHeadCopy is what the revision holds at the path of a policy that is usable on disk. It is read for disclosure
// only, so whatever it holds, a policy that is not a usable one included, refuses nothing. What still refuses is what the
// working tree or the repository says about the named path itself: a spelling in another case, a path inside a
// submodule checkout, which this repository can never commit.
func readHeadCopy(dir, revision string, source policyref.Ref, path string, ports scanPorts) (projectconfig.CommittedCopy, error) {
	head, err := ports.readCommitted(dir, revision, path)
	if err != nil || !source.Explicit {
		return head, err
	}
	if head.Kind == projectconfig.CopyAbsent && insideSubmodule(dir, revision, path) {
		_, refusal := ports.loadExplicit(dir, revision, path)
		return head, refusal
	}
	return head, explicitOnDiskRefusal(dir, revision, path)
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
