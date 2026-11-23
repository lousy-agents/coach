package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// judgeSelectedPolicy passes a usable policy on disk with the roots of the bytes read, and says how it relates to the
// analyzed revision. Two states outrank the file: it lies inside a submodule checkout, where the superproject can
// never commit it, or the revision holds the policy only in another case, which the revision still decides.
func judgeSelectedPolicy(run gitrepo.Runner, dir, revision string, selection projectconfig.DiskSelection, config projectconfig.Config, policy policyref.Ref) (policyJudgment, error) {
	evidence := readHeadEvidence(run, dir, revision, selection)
	if evidence.submoduleMessage != "" {
		check := projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing}
		return policyJudgment{path: selection.Path, check: check.WithCause(evidence.submoduleMessage, projectreadiness.CauseInSubmodule)}, nil
	}
	if evidence.copy == headHoldsNoFile {
		renamed, unanswered := wrongCaseAtRevision(run, dir, revision, selection.Path)
		if renamed != nil {
			return judgmentOfWrongCase(renamed, selection.Path, policy)
		}
		evidence.unanswered = evidence.unanswered || unanswered
	}
	judgment := policyJudgment{path: selection.Path, check: projectreadiness.Check{State: projectreadiness.Pass}, roots: config.Roots}
	if !policy.Explicit && selection.Path == projectconfig.DefaultPath {
		judgment.warnings = append(judgment.warnings, projectreadiness.Warning{Code: projectreadiness.WarnProjectConfigDeprecated})
	}
	if code := evidence.warningCode(); code != "" {
		judgment.warnings = append(judgment.warnings, projectreadiness.Warning{Code: code, Root: selection.Path})
	}
	return judgment, nil
}
