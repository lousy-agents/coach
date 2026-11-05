package projectcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const validPolicy = `{"schema_version":"1","roots":["."]}`

func TestRunAfterPolicyErrorReportsTheVerdictItWasGivenInsteadOfReadingTheWorkingTreeAgain(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.go", "package a\n")
	gitfixture.CommitFile(t, repo, "pol.json", validPolicy)
	given := &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Cause: projectconfig.CauseAbsent, Remedy: projectreadiness.RemedyNone, FromWorkingTree: true, UnreadStagedEdit: true}

	result, err := RunAfterPolicyError(repo, "HEAD", "pol.json", given)

	if err != nil {
		t.Fatalf("RunAfterPolicyError: %v", err)
	}
	policy := result.Checks.Policy
	if policy.Code != projectreadiness.GapPolicyMissing || policy.Remedy != projectreadiness.RemedyNone || policy.DecidedOnDisk {
		t.Errorf("policy check = %+v; want the given verdict (policy_missing, no remedy, not decided on disk) although pol.json is a regular file on disk", policy)
	}
}

func TestRunAfterPolicyErrorJudgesTheAnalyzedRevisionForAnErrorTheWorkingTreeDidNotProduce(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.go", "package a\n")
	if err := os.WriteFile(filepath.Join(repo, "pol.json"), []byte(validPolicy), 0o644); err != nil {
		t.Fatalf("write pol.json: %v", err)
	}
	given := &projectconfig.ConfigError{Kind: projectconfig.KindUncommitted, Remedy: projectreadiness.RemedyNone}

	result, err := RunAfterPolicyError(repo, "HEAD", "pol.json", given)

	if err != nil {
		t.Fatalf("RunAfterPolicyError: %v", err)
	}
	if got := result.Checks.Policy; got.Code != projectreadiness.GapPolicyMissing || got.Remedy != projectreadiness.RemedyNone {
		t.Errorf("policy check = %+v; want policy_missing with no remedy, because the file is on disk and not committed at the revision", got)
	}
}
