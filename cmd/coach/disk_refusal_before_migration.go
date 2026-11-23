package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// refusedOnDiskBeforeMigration is the refusal of a discovered policy the working tree cannot supply. It is asked before
// the offer, so that a scan that is about to stop does not ask the user to copy a file first.
func refusedOnDiskBeforeMigration(dir, revision string, f codesignalFlags, ports scanPorts) error {
	if f.namedPolicy.Explicit {
		return nil
	}
	selection, err := ports.discover(dir, revision)
	if err != nil {
		return nil
	}
	if _, err := judgeDiscoveredOnDisk(dir, revision, selection, ports); err != nil {
		return withReadinessForLanguage(err, dir, revision, policyref.Ref{}, f.projectLanguage)
	}
	return nil
}
