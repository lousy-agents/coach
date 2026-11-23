package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// wrongCaseAtRevision is the refusal for a policy the revision holds only in another case, or nil when it holds it in
// none. The second result is set when git could not say, in which case the policy is left to pass with the warning that
// the revision could not be compared.
func wrongCaseAtRevision(run gitrepo.Runner, dir, revision, path string) (renamed error, unanswered bool) {
	renamed = projectconfig.WrongCaseAtRevision(run, dir, revision, path)
	if renamed == nil {
		return nil, false
	}
	if projectconfig.GitSilence(renamed, "coach codesignal --check-project", path) != nil {
		return nil, true
	}
	return renamed, false
}

func judgmentOfWrongCase(renamed error, path string, policy policyref.Ref) (policyJudgment, error) {
	check, _, err := policyGapFrom(renamed)
	if err != nil {
		return policyJudgment{}, err
	}
	return policyJudgment{path: path, check: wordedForThePolicy(check, policy, path)}, nil
}
