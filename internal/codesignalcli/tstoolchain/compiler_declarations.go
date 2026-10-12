package tstoolchain

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// declarationMismatches lists the selected roots whose manifest disagrees
// with the winning compiler. A winning non-project origin disagrees with any
// differing declaration; when the project origin itself wins, only a stale
// exact pin disagrees -- range satisfaction is never evaluated (epic #280,
// owner decision D4).
func (a compilerAggregate) declarationMismatches() []projectreadiness.DeclarationMismatch {
	if a.Winner == nil {
		return nil
	}
	projectWon := a.Winner.Origin == OriginProject
	var mismatches []projectreadiness.DeclarationMismatch
	for _, declaration := range a.project.declarations {
		if declaration.declared == a.Winner.Version {
			continue
		}
		if projectWon && !IsExactVersion(declaration.declared) {
			continue
		}
		mismatches = append(mismatches, projectreadiness.DeclarationMismatch{Root: declaration.root, Declared: declaration.declared})
	}
	return mismatches
}

func (a compilerAggregate) namedDeclaration() string {
	if a.project.rejectedDeclaration != "" {
		return a.project.rejectedDeclaration
	}
	if len(a.project.declarations) > 0 {
		return a.project.declarations[0].declared
	}
	return ""
}
