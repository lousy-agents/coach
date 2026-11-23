package main

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// diskQuestion is what the working tree is asked before anything the revision commits is consulted: which policy file
// it holds, for a discovered policy or for the one named. A refusal ends the scan whatever the revision commits, so no
// committed copy and no other policy file stands in for an entry that is there and cannot be used.
type diskQuestion struct {
	namedPath string
	// matters says whether a refusal at the path is the scan's to report. A file the scan would not have read anyway
	// is not: a root project.json that is not committed is ignored whether it is usable or not.
	matters func(path string) bool
	refuse  func(refusal *projectconfig.DiskPolicyRefusal) error
}

// judgeDiscoveredOnDisk asks the working tree for the discovered policy. The canonical path is always the scan's to
// judge, and the root project.json only when discovery selected it, which it does only for a committed one.
func judgeDiscoveredOnDisk(dir, revision string, discovery projectconfig.Selection, ports scanPorts) (projectconfig.DiskSelection, error) {
	if err := discovery.SubmoduleRefusal(); err != nil {
		return projectconfig.DiskSelection{}, err
	}
	return judgeOnDisk(dir, diskQuestion{
		matters: func(path string) bool { return path == projectconfig.CanonicalPath || path == discovery.Path },
		refuse: func(refusal *projectconfig.DiskPolicyRefusal) error {
			if refusal.Path == projectconfig.CanonicalPath {
				return refusal.ScanError(false, projectconfig.UnmergedInIndex(dir, refusal.Path), discovery.Fatal)
			}
			return refusal.ScanError(false, projectconfig.UnmergedInIndex(dir, refusal.Path), nil)
		},
	}, ports)
}

// judgeNamedOnDisk asks the working tree for the policy the user named. A path that cannot name a policy is left to
// the loader, whose message for it is unchanged.
func judgeNamedOnDisk(dir, revision string, named policyref.Ref, ports scanPorts) (projectconfig.DiskSelection, error) {
	if projectconfig.ValidatePath(named.Path) != nil {
		return projectconfig.DiskSelection{}, nil
	}
	return judgeOnDisk(dir, diskQuestion{
		namedPath: named.Path,
		matters:   func(string) bool { return true },
		refuse: func(refusal *projectconfig.DiskPolicyRefusal) error {
			_, loadErr := ports.loadExplicit(dir, revision, refusal.Path)
			var configErr *projectconfig.ConfigError
			switch {
			case loadErr != nil && !errors.As(loadErr, &configErr):
				return loadErr
			case loadErr != nil && refusal.Cause() != "" && refusal.CommittedEntryFits(loadErr):
				// The loader judged the same entry on disk, with what the index and the revision add to it.
				return refusal.CauseWorded(loadErr)
			}
			return refusal.ScanError(true, projectconfig.UnmergedInIndex(dir, refusal.Path), loadErr)
		},
	}, ports)
}

func judgeOnDisk(dir string, question diskQuestion, ports scanPorts) (projectconfig.DiskSelection, error) {
	selection, err := ports.selectOnDisk(dir, question.namedPath)
	if err == nil && selection.Path != "" {
		_, err = projectconfig.UsableDiskPolicy(selection)
	}
	if err == nil {
		return selection, nil
	}
	refusal := projectconfig.ClassifyDiskPolicyError(selection.Path, err)
	switch {
	case refusal == nil:
		return projectconfig.DiskSelection{}, err
	case !question.matters(refusal.Path):
		return projectconfig.DiskSelection{}, nil
	}
	return projectconfig.DiskSelection{}, question.refuse(refusal)
}
