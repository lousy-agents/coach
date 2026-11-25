package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// policyJudgment is what the check concluded about the policy: the path it judged, the check itself, and, for a policy
// that passed, the roots it gave the other checks and the warnings that say how it relates to the analyzed revision.
type policyJudgment struct {
	path     string
	check    projectreadiness.Check
	roots    []string
	warnings []projectreadiness.Warning
}

// judgePolicy judges the policy from the working tree: a usable regular file there is the policy, whatever the
// analyzed revision and the index hold for its path (ADR-009). Only a path with no usable file on disk asks the revision
// and the index which way out there is. A root project.json that discovery falls back to is a policy only when it is a
// valid one, unless the analyzed revision selects it too, in which case it is the policy and the gap is its refusal.
func judgePolicy(run gitrepo.Runner, dir, revision string, policy policyref.Ref) (policyJudgment, error) {
	if _, err := gitrepo.RepositoryRootWith(run, dir); err != nil {
		return policyJudgment{}, err
	}
	namedPath, _ := policy.NamedPath()
	discovered := namedPath == ""
	selection, err := projectconfig.SelectOnDiskForCheck(dir, namedPath)
	if respelled := asWrongCase(err); respelled != nil {
		return judgmentOfWrongCase(respelled, wrongCasePath(namedPath), policy)
	}
	if err != nil {
		failedAt := selectionFailurePath(dir, namedPath)
		if discovered && failedAt == projectconfig.DefaultPath {
			return judgeRootProjectThatCannotBeUsed(run, dir, revision, policy, err)
		}
		return judgmentOfRefusal(failedAt, err, policy)
	}
	if selection.Path == "" {
		return judgeAbsentPolicy(run, dir, revision, policy)
	}
	config, err := projectconfig.UsableDiskPolicy(selection)
	if err != nil {
		if discovered && selection.Path == projectconfig.DefaultPath {
			return judgeRootProjectThatCannotBeUsed(run, dir, revision, policy, err)
		}
		return judgmentOfRefusal(selection.Path, err, policy)
	}
	return judgeSelectedPolicy(run, dir, revision, selection, config, policy)
}

// wordedForThePolicy is check worded as the scan words it: a policy Coach found is the default policy, and not a
// --project-config the user never passed.
func wordedForThePolicy(check projectreadiness.Check, policy policyref.Ref, path string) projectreadiness.Check {
	if !policy.JudgedOnDisk() {
		check.CauseText = projectconfig.WordedAsTheDefaultPolicy(check.CauseText, path)
	}
	return check
}
