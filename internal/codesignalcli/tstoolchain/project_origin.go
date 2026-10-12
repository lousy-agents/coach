package tstoolchain

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

type projectRootOutcome struct {
	root         string
	candidate    string
	finding      string
	declaration  string
	manifestDir  string
	installed    bool
	disqualified bool
	unreadable   bool
	ambiguous    bool
}

type projectOriginContext struct {
	rootFindings             []projectreadiness.RootFinding
	someRootsResolvedNothing bool
	declarations             []rootDeclaration
	rejectedDeclaration      string
}

type rootDeclaration struct {
	root     string
	declared string
}

func evaluateProjectOrigin(worktreeRoot string, roots []string) (originEvaluation, projectOriginContext) {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	summary := projectRootSummary{
		projectOriginContext: projectOriginContext{rootFindings: make([]projectreadiness.RootFinding, 0, len(roots))},
	}
	for _, root := range roots {
		summary.absorb(resolveProjectRoot(worktreeRoot, root))
	}
	summary.close(len(roots))
	return summary.evaluation(), summary.projectOriginContext
}
