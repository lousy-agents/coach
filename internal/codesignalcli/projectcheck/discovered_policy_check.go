package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// checkDiscoveredPolicy is checkPolicy for a policy Coach found, worded as the scan words it: the default policy, and
// not a --project-config the user never passed.
func checkDiscoveredPolicy(run gitrepo.Runner, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	check, roots, err := discoveredPolicyCheck(run, dir, revision, policyPath)
	check.CauseText = projectconfig.WordedAsTheDefaultPolicy(check.CauseText, check.PolicyCause, policyPath)
	return check, roots, err
}

// discoveredPolicyCheck is the check, which also names a policy the revision holds in another case. That is not a gap
// to author over: the policy is there, at a name Coach does not read.
func discoveredPolicyCheck(run gitrepo.Runner, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	check, roots, err := checkPolicy(run, dir, revision, policyPath)
	if err != nil || check.Code != projectreadiness.GapPolicyMissing {
		return check, roots, err
	}
	if renamed := projectconfig.WrongCaseAtRevision(run, dir, revision, policyPath); renamed != nil {
		return gapForWrongCase(renamed, policyPath)
	}
	return gapForDiscoveredPolicyOnDisk(run, dir, revision, policyPath, check)
}

// gapForDiscoveredPolicyOnDisk is the result for a discovered policy that the revision does not hold. When
// the working tree holds something at the path, that decides the gap as it does for a policy named with
// --project-config: authoring refuses a target that exists, so a file that is not committed is committed and
// not written again, and a path the disk cannot use says why. With nothing on disk, missing is the gap and a
// new policy fixes it.
func gapForDiscoveredPolicyOnDisk(run gitrepo.Runner, dir, revision, policyPath string, missing projectreadiness.Check) (projectreadiness.Check, []string, error) {
	status, err := onDiskStatus(run, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if leavesNoPolicyToCommit(status) {
		return missing, nil, nil
	}
	return explicitPolicyCheck(run, status, dir, revision, policyPath)
}

// leavesNoPolicyToCommit is whether the working tree and the index hold nothing at the discovered path that a person
// would commit or lose, so that the gap is the policy that is missing: the path is free for a new policy
// (ConfigError.FreeForANewPolicy, which asks the index, a conflict, a mark and a submodule as well as the disk).
func leavesNoPolicyToCommit(status *projectconfig.ConfigError) bool {
	return status != nil && status.FreeForANewPolicy()
}

// gapForWrongCase is the result for a discovered policy that is missing where Coach reads it: the gap that
// names the other case it is committed in. When git could not say whether it is, the result is the
// operational error that names the question, which is how every question git could not answer is reported.
func gapForWrongCase(renamed error, policyPath string) (projectreadiness.Check, []string, error) {
	if silence := projectconfig.GitSilence(renamed, "coach codesignal --check-project", policyPath); silence != nil {
		return projectreadiness.Check{}, nil, silence
	}
	return policyGapFrom(renamed)
}
