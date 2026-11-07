package projectcheck

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// gitThatHoldsOnlyCfgPolicy is the whole of git for the policy check of a repository whose revision holds
// cfg/p.json and nothing else: a revision that resolves, no entry at any other path, no submodule, and
// core.ignorecase as given. Any other question is an error that names it.
func gitThatHoldsOnlyCfgPolicy(ignoreCase string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "cat-file", args[0] == "ls-files":
			return nil, nil
		case args[0] == "config":
			return []byte(ignoreCase + "\n"), nil
		case args[0] == "ls-tree" && slices.Contains(args, "--name-only"):
			return namesInTheListing(args[len(args)-1]), nil
		case args[0] == "ls-tree" && slices.Contains(args, "-z"):
			return nil, nil
		}
		return nil, errors.New("gitThatHoldsOnlyCfgPolicy: unexpected git question: " + strings.Join(args, " "))
	}
}

func namesInTheListing(last string) []byte {
	if strings.TrimSuffix(strings.TrimPrefix(last, ":(top,literal)"), "/") == "cfg" {
		return []byte("p.json\x00")
	}
	return []byte("cfg\x00")
}

// A policy on disk under a name that differs only by case from what the revision holds is not missing: the scan
// refuses it as invalid, and the check says policy_invalid with the case cause, as it does for the revision's own
// spelling. The state exists only on a file system that folds case, so it is specified from the answers git gives.
func TestAnUncommittedPolicyInAnotherCaseIsInvalidWithTheCaseCause(t *testing.T) {
	withReadinessGit(t, gitThatHoldsOnlyCfgPolicy("true"))

	check, _, err := checkPolicyOnDisk("repo", "HEAD", "cfg/P.json")

	if err != nil || check.Code != projectreadiness.GapPolicyInvalid || check.PolicyCause != projectreadiness.CauseNameCaseDiffers {
		t.Fatalf("checkPolicyOnDisk = %+v, %v; want policy_invalid with the cause %q", check, err, projectreadiness.CauseNameCaseDiffers)
	}
	if check.Remedy != projectreadiness.RemedyNone || !check.DecidedOnDisk {
		t.Errorf("remedy %d, decided on disk %v; want no remedy, and a verdict the disk decided", check.Remedy, check.DecidedOnDisk)
	}
}

func TestAnUncommittedPolicyWhereCasesAreToldApartIsMissingNotInvalid(t *testing.T) {
	withReadinessGit(t, gitThatHoldsOnlyCfgPolicy("false"))

	check, _, err := checkPolicyOnDisk("repo", "HEAD", "cfg/P.json")

	if err != nil || check.Code != projectreadiness.GapPolicyMissing || check.PolicyCause != projectreadiness.CauseNotCommitted {
		t.Errorf("checkPolicyOnDisk = %+v, %v; want policy_missing with the cause %q", check, err, projectreadiness.CauseNotCommitted)
	}
}

func TestOnlyTheCaseCauseMakesAnUncommittedPolicyInvalid(t *testing.T) {
	for cause, want := range map[projectreadiness.PolicyCause]string{
		projectreadiness.CauseNameCaseDiffers: projectreadiness.GapPolicyInvalid,
		projectreadiness.CauseNotCommitted:    projectreadiness.GapPolicyMissing,
		projectreadiness.CauseInSubmodule:     projectreadiness.GapPolicyMissing,
		projectreadiness.CauseUndetermined:    projectreadiness.GapPolicyMissing,
	} {
		if got := codeForUncommittedCause(cause); got != want {
			t.Errorf("codeForUncommittedCause(%q) = %q, want %q", cause, got, want)
		}
	}
}
