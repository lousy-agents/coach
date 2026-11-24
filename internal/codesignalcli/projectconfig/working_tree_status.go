package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// workingTreeStatus is ExplicitOnDiskStatus for a path whose spelling the analyzed revision has already settled.
func workingTreeStatus(run gitrepo.Runner, dir, revision, repoPath string) (*ConfigError, error) {
	err := requireOnDisk(run, dir, repoPath)
	if err == nil {
		return nil, nil
	}
	var onDisk *ConfigError
	if !errors.As(err, &onDisk) {
		return nil, err
	}
	named, err := namedByTheRevision(run, dir, revision, repoPath, onDisk)
	if err != nil {
		return nil, err
	}
	found, err := namedForACommittedBlocker(run, dir, revision, repoPath, named)
	if err != nil {
		return nil, err
	}
	judgment := judgeRestore(run, dir, revision, repoPath)
	judged := *found
	if judgment.failure != nil && found.Kind == KindMissing {
		// A failed question changes the outcome only where an offer depends on it. For a path the
		// working tree found unusable no offer is made, so the failure is not an operational error.
		judged.GitFailure = judgment.failure
	}
	offer, offered := restoreOfferFor(judgment, revision, found, repoPath)
	if !offered {
		return &judged, nil
	}
	return annotated(&judged, offer), nil
}
