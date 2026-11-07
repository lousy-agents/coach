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
	if base == "" {
		return nil
	}
	mergeBase, err := mergeBaseOf(run, dir, base)
	if err != nil {
		return nil
	}
	revision := mergeBaseRevision(base, mergeBase)
	missing, err := missingObjectsOf(run, dir, revision.name)
	if err != nil {
		return &OperationalError{Message: fmt.Sprintf("could not tell whether %s has the objects the changed files are compared with (%s), so the diff is not run: a file that cannot be compared is recorded as unreadable, and the run ends without having analyzed it", revision.label, TerminalSafe(err.Error())), Cause: err}
	}
	if !missing.any() {
		return nil
	}
	return &OperationalError{Message: fmt.Sprintf("this repository is a partial clone, and %s has %s not in it, so the files that changed cannot be compared with their earlier versions, and Coach does not fetch objects%s", revision.label, missing.phrase(), advice(FetchAdvice(run, dir, revision.name, "")))}
}

func advice(fetch string) string {
	if fetch == "" {
		return ""
	}
	return "; " + fetch
}
