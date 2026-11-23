package projectcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func judged(t *testing.T, repo string, policy policyref.Ref) policyJudgment {
	t.Helper()
	judgment, err := judgePolicy(readinessGit(), repo, "HEAD", policy)
	if err != nil {
		t.Fatalf("judgePolicy: %v", err)
	}
	return judgment
}

func requireNoWayOutAndTheCause(t *testing.T, check projectreadiness.Check, code string, cause projectreadiness.PolicyCause) {
	t.Helper()
	if check.Code != code || check.PolicyCause != cause || check.Remedy != projectreadiness.RemedyNone {
		t.Errorf("check = %+v; want %s, no remedy, cause %q", check, code, cause)
	}
}

// A regular file that cannot be used is a policy the check cannot name a way out for, because authoring refuses a path
// that exists; the cause says nothing the check established except for a file the process could not read.
func TestARegularFileOnDiskThatIsNoPolicyIsInvalidWithNoWayOut(t *testing.T) {
	cases := map[string]string{
		"content that is not a policy": "not json",
		"a file over the size limit":   strings.Repeat("x", projectconfig.MaxBytes+1),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			repo := gitfixture.Init(t)
			gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
			writeWorktreeFile(t, repo, "cfg/p.json", content)

			judgment := judged(t, repo, policyref.Named("cfg/p.json"))

			requireNoWayOutAndTheCause(t, judgment.check, projectreadiness.GapPolicyInvalid, projectreadiness.CauseUndetermined)
			if judgment.roots != nil || judgment.warnings != nil {
				t.Errorf("roots %v, warnings %v; want none from a file that was refused", judgment.roots, judgment.warnings)
			}
		})
	}
}

// A root project.json that discovery falls back to counts only when it is valid, so an invalid one leaves the policy
// missing, as it does when the revision commits it.
func TestAnInvalidRootProjectJSONIsNoDiscoveredPolicy(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, projectconfig.DefaultPath, "not json")

	judgment := judged(t, repo, policyref.Discovered(""))

	if judgment.check.Code != projectreadiness.GapPolicyMissing || judgment.check.Remedy != projectreadiness.RemedyAuthor || judgment.warnings != nil {
		t.Errorf("check = %+v, warnings %v; want policy_missing answered with a new policy and no deprecation warning", judgment.check, judgment.warnings)
	}
}

func TestAValidRootProjectJSONIsTheDiscoveredPolicyWithTheDeprecationWarning(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, projectconfig.DefaultPath, validPolicy)

	judgment := judged(t, repo, policyref.Discovered(""))

	if judgment.path != projectconfig.DefaultPath || judgment.check.State != projectreadiness.Pass || len(judgment.warnings) != 1 || judgment.warnings[0].Code != projectreadiness.WarnProjectConfigDeprecated {
		t.Errorf("judgment = %+v; want project.json passing with %q", judgment, projectreadiness.WarnProjectConfigDeprecated)
	}
}

func TestANamedRootProjectJSONIsNotWarnedAboutAsDeprecated(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, projectconfig.DefaultPath, validPolicy)

	judgment := judged(t, repo, policyref.Named(projectconfig.DefaultPath))

	if judgment.check.State != projectreadiness.Pass || judgment.warnings != nil {
		t.Errorf("judgment = %+v; want a pass and no warning, because the user named it", judgment)
	}
}

// The revision decides what a path names: a file on disk at the spelling Coach reads is refused while the revision holds
// the policy only in another case.
func TestAFileOnDiskIsRefusedForItsNameWhileTheRevisionHoldsThePolicyInAnotherCase(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/P.json", validPolicy)
	writeWorktreeFile(t, repo, "cfg/p.json", validPolicy)
	if entries, err := os.ReadDir(filepath.Join(repo, "cfg")); err != nil || len(entries) != 2 {
		t.Skip("this file system folds case, so cfg/p.json and cfg/P.json are one file")
	}

	judgment := judged(t, repo, policyref.Named("cfg/p.json"))

	requireNoWayOutAndTheCause(t, judgment.check, projectreadiness.GapPolicyInvalid, projectreadiness.CauseNameCaseDiffers)
}
