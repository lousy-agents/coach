package gitrepo

import "fmt"

// refuseWhenTheMergeBaseLacksObjects is the check for a git that never fetches. A scan of a diff compares each
// changed file with its version at the merge base, and in a partial clone that version may not be there. Left
// to the scan, the file is recorded as unreadable and the run ends with exit 0 having analyzed none of the
// change, which a job that gates on the exit status reads as a pass. HEAD's own objects are read, and reported
// when missing, by the snapshot, so only the merge base is asked about.
//
// A base that has no merge base is left to the scan, which says so in its own words; once the merge base is
// known, a question that cannot be answered is a refusal, because the scan would go on without the answer.
func refuseWhenTheMergeBaseLacksObjects(run Runner, dir, base string) error {
	revision, found := mergeBaseRevisionOf(run, dir, base)
	if !found {
		return nil
	}
	missing, err := missingObjectsOf(run, dir, revision.name)
	if err != nil {
		return &OperationalError{Message: fmt.Sprintf("could not tell whether %s has the objects the changed files are compared with (%s), so the diff is not run: a file that cannot be compared is recorded as unreadable, and the run ends without having analyzed it", revision.label, TerminalSafe(err.Error())), Cause: err}
	}
	if !missing.any() {
		return nil
	}
	return &OperationalError{Message: fmt.Sprintf("this repository is a partial clone, and %s has %s not in it, so the files that changed cannot be compared with their earlier versions, and Coach does not fetch objects%s", revision.label, missing.phrase(), advice(FetchAdviceFor(run, dir, revision.name, "HEAD")))}
}

func advice(fetch string) string {
	if fetch == "" {
		return ""
	}
	return "; " + fetch
}

// refuseWhenHeadLacksTrees is the check for the trees of HEAD, which every read of the revision begins with: a
// clone made with --filter=tree:0 has the commit and not its root tree, and `ls-tree` fails on it with "not a
// tree object". Blobs are not asked about: the snapshot reads, and reports, the ones it needs, and a clone with
// no checkout can be checked for a policy with only some of them. A probe that cannot be answered is left to the
// scan, which says why HEAD cannot be read; an empty repository has no HEAD, and is not a partial clone to blame.
//
// For a diff the batch lists the merge base as well, which the clone made without trees lacks too: one paste then
// brings in what both reads need.
func refuseWhenHeadLacksTrees(run Runner, dir, base string) error {
	missing, err := treesMissingOf(run, dir, "HEAD")
	if err != nil || !missing.any() {
		return nil
	}
	return &OperationalError{Message: fmt.Sprintf("this repository is a partial clone, and HEAD has %s not in it, so its files cannot be listed, and Coach does not fetch objects%s", missing.phrase(), advice(FetchAdviceFor(run, dir, revisionsToFetch(run, dir, base)...)))}
}

// revisionsToFetch are the revisions the scan reads, which one paste brings in together.
func revisionsToFetch(run Runner, dir, base string) []string {
	return revisionNames(revisionsRead(run, dir, base))
}
