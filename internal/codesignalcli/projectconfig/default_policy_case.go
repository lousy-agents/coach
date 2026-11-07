package projectconfig

import (
	"fmt"
	"slices"
	"strings"
)

// namesTheDefaultPolicyInAnotherCase is whether the user named the default policy's path in another case. A scan
// without --project-config reads only the default spelling, so for that path the other callers are the ones the advice
// has to keep working.
func namesTheDefaultPolicyInAnotherCase(repoPath string, chooser pathChooser) bool {
	return chooser == chosenByTheUser && repoPath != CanonicalPath && strings.EqualFold(repoPath, CanonicalPath)
}

// whereToRecord is the path git has to record the policy at. It is the path Coach reads, except for the default
// policy's path named in another case: recording the policy at that spelling would make every scan that passes no flag
// refuse it, so it is recorded at the default spelling, which both callers read.
func whereToRecord(repoPath string, chooser pathChooser) string {
	if namesTheDefaultPolicyInAnotherCase(repoPath, chooser) {
		return CanonicalPath
	}
	return repoPath
}

// keepsTheDefaultSpelling is whether the default policy's path was named in another case while the revision commits
// the default spelling: nothing is renamed then, and the default spelling is never the one to delete.
func keepsTheDefaultSpelling(repoPath string, others []string, chooser pathChooser) bool {
	return namesTheDefaultPolicyInAnotherCase(repoPath, chooser) && slices.Contains(others, CanonicalPath)
}

// keepTheDefaultPolicy is the advice for the default policy's path named in another case, when the default spelling is
// committed: pass that spelling or leave the flag off, and delete the other spellings if there are any. No rename is
// offered, because the spelling typed is not one a scan without the flag reads.
func keepTheDefaultPolicy(others []string, truncated bool, repoPath string) string {
	way := fmt.Sprintf("pass the committed spelling, %q, or leave --project-config off, which reads that spelling by itself", CanonicalPath)
	rest := slices.DeleteFunc(slices.Sorted(slices.Values(others)), func(spelling string) bool { return spelling == CanonicalPath })
	if len(rest) == 0 && !truncated {
		return fmt.Sprintf("the policy is committed as %q, but Coach reads %q, so it is not read: %s; making git record it as %q would make every scan without the flag refuse it", CanonicalPath, repoPath, way, repoPath)
	}
	deleted := "the others"
	named := quotedList(slices.Sorted(slices.Values(others)))
	if truncated {
		named += ", and more"
	} else {
		deleted = quotedList(rest)
	}
	return fmt.Sprintf("the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: keep %q, which a scan without --project-config reads by itself, and delete %s from the repository, then commit and run again with that spelling passed or --project-config left off", named, repoPath, CanonicalPath, deleted)
}
