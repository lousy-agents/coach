package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// readDiskPolicy is what the working tree selects for the policy source: the named file alone, or whichever of the
// discovery candidates is a regular file. A file that is there and cannot be used is a refusal, not a missing policy.
func readDiskPolicy(dir string, source policyref.Ref, ports scanPorts) diskPolicy {
	namedPath, _ := source.NamedPath()
	selection, err := ports.selectOnDisk(dir, namedPath)
	if err != nil {
		return diskPolicy{refusal: projectconfig.ClassifyDiskPolicyError(unreadablePolicyPath(dir, namedPath, ports), err)}
	}
	if refusal := committedEntryRefusal(dir, source); refusal != nil {
		return diskPolicy{refusal: refusal}
	}
	if selection.Path == "" {
		return diskPolicy{}
	}
	if _, err := projectconfig.UsableDiskPolicy(selection); err != nil {
		return diskPolicy{refusal: projectconfig.ClassifyDiskPolicyError(selection.Path, err)}
	}
	return diskPolicy{selection: selection}
}

// unreadablePolicyPath names the file a failed selection stopped at. SelectOnDisk reports the error and not the path,
// and it stops at the first candidate that is a regular file it cannot read, so asking for the canonical file alone
// tells whether that was the one.
func unreadablePolicyPath(dir, namedPath string, ports scanPorts) string {
	if namedPath != "" {
		return namedPath
	}
	if _, err := ports.selectOnDisk(dir, projectconfig.CanonicalPath); err != nil {
		return projectconfig.CanonicalPath
	}
	return projectconfig.DefaultPath
}

// committedEntryRefusal is the refusal for a committed discovered policy whose path holds, on disk, an entry that
// cannot be a policy. Without it that path reads as missing, and the scan says a file is missing that is there.
func committedEntryRefusal(dir string, source policyref.Ref) *projectconfig.DiskPolicyRefusal {
	if source.Explicit || source.Path == "" {
		return nil
	}
	return projectconfig.UnusableEntryOnDisk(dir, source.Path)
}
