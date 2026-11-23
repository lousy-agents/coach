package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// judgeRootProjectThatCannotBeUsed is the judgment for a discovered policy whose root project.json is a regular file on
// disk that cannot be used. Where the analyzed revision selects that same file, it is the policy and the file is
// refused, as a canonical one is; where it does not, nothing on disk is a policy and the path is judged absent.
func judgeRootProjectThatCannotBeUsed(run gitrepo.Runner, dir, revision string, policy policyref.Ref, cause error) (policyJudgment, error) {
	selection, err := projectconfig.Discover(dir, revision)
	if err != nil || selection.Path != projectconfig.DefaultPath {
		return judgeAbsentPolicy(run, dir, revision, policy)
	}
	judgment, err := judgmentOfRefusal(projectconfig.DefaultPath, cause, policy)
	judgment.warnings = []projectreadiness.Warning{{Code: projectreadiness.WarnProjectConfigDeprecated}}
	return judgment, err
}
