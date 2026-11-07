package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"

// stagedNonFileOverCommittedOffer is for a committed policy with a symbolic link, or another entry that is not
// a regular file, staged over it. Keeping what the index holds writes the link back to disk, and the scan
// refuses a link on the next run, so that step is not offered and the text says why. Only the restore from the
// revision is printed: it brings back the committed copy to the index and the file, which the scan reads. A file
// in the way is moved before it.
func stagedNonFileOverCommittedOffer(revision, repoPath string, blocked bool) restoreOffer {
	drop := restoreAdvice(revision, repoPath)
	if blocked {
		drop = "move the file that blocks the path, then " + drop
	}
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseStagedNotAFile,
		text: ". It is committed at the analyzed revision, but the index holds a symbolic link, or another entry that is not a regular file, staged over it, which a policy cannot be and the scan refuses, so restoring what the index holds would recreate it and no keep step is offered; " +
			"review the staged entry with git diff --cached, then run again; to drop it and bring back the committed copy instead, " + drop,
	}
}
