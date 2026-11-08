package projectconfig

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// namedByTheRevision is the refusal to report for a path the working tree cannot use as a policy. When the
// working tree finds a link or an entry that is not a regular file, and the analyzed revision commits the path
// as a link, a directory, a submodule or another entry that is not a file, the revision's refusal is the more
// specific one: it holds whatever is on disk, so the word is the same whether or not a file is there. Any other
// refusal of the disk stands. The error return is a question git could not answer, never a verdict.
func namedByTheRevision(run gitrepo.Runner, dir, revision, repoPath string, onDisk *ConfigError) (*ConfigError, error) {
	if !refusedForWhatIsOnDisk(onDisk) {
		return onDisk, nil
	}
	committed, err := NonFileAtRevision(run, dir, revision, repoPath)
	if err != nil || committed == nil {
		return onDisk, err
	}
	return committed, nil
}

// refusedForWhatIsOnDisk is whether the disk refused the path itself, as a link or as an entry that is not a
// regular file, rather than a parent of it or a file it cannot read.
func refusedForWhatIsOnDisk(refusal *ConfigError) bool {
	return refusal.PolicyCause == projectreadiness.CauseSymbolicLink || refusal.PolicyCause == projectreadiness.CauseNotARegularFile
}
