package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"

// stagedNonFileOverCommittedOffer is for a committed policy with a symbolic link, or another entry that is not
// a regular file, staged over it. Restoring what the index holds writes the link back to disk, and the scan
// refuses a link on the next run, so that restore is not offered and the text says why. Two ways out are told. The
// change is kept by staging a regular file at the path in place of the link, which is the one step that ends in
// running again, because reviewing the entry changes nothing and the same message would come back. It is dropped
// by the restore from the revision, the only command printed: it brings back the committed copy to the index and
// the file, which the scan reads. A file in the way is moved before either.
func stagedNonFileOverCommittedOffer(revision, repoPath string, blocked bool) restoreOffer {
	keep := behindTheMove("stage a regular file at the path in place of the link", blocked)
	drop := behindTheMove(restoreAdvice(revision, repoPath), blocked)
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseStagedNotAFile,
		text: ". It is committed at the analyzed revision, but the index holds a symbolic link, or another entry that is not a regular file, staged over it, which a policy cannot be and the scan refuses, so restoring what the index holds would recreate it and that restore is not offered; " +
			"review the staged entry with git diff --cached; to keep the change, " + keep + ", then run again; to drop it and bring back the committed copy instead, " + drop,
	}
}
