package codesignalcli

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func renderProjectScopeSection(b *strings.Builder, scope *codesignal.ProjectScopeReport) {
	if scope == nil {
		return
	}
	b.WriteString("\nProject scope:\n")
	renderInclusionRule(b, scope.InclusionRule)
	fmt.Fprintf(b, "pattern_set: %s\n", scope.PatternSet)
	renderProjectScopeRevision(b, "head", scope.Head)
	if scope.Base != nil {
		renderProjectScopeRevision(b, "base", *scope.Base)
	}
}

func renderInclusionRule(b *strings.Builder, rule string) {
	switch rule {
	case projectmodel.InclusionRuleTSConfigIncludesNoTestClassification:
		fmt.Fprintf(b, "inclusion: TypeScript files are those the snapshot tsconfig includes; production versus test classification is not applied (see #281).\n")
	default:
		fmt.Fprintf(b, "inclusion: %s\n", rule)
	}
}

func renderProjectScopeRevision(b *strings.Builder, label string, rev codesignal.ProjectScopeRevisionReport) {
	fmt.Fprintf(b, "%s_revision: %s\n", label, rev.Revision)
	for _, root := range rev.Roots {
		fmt.Fprintf(b, "  root: %s  candidate_files: %d  analyzed_files: %d\n", root.Root, root.CandidateFiles, root.AnalyzedFiles)
	}
	if len(rev.MatchedLayers) > 0 {
		fmt.Fprintf(b, "matched_layers: %s\n", strings.Join(rev.MatchedLayers, ", "))
	}
	if len(rev.UnmatchedLayers) > 0 {
		fmt.Fprintf(b, "unmatched_layers: %s\n", strings.Join(rev.UnmatchedLayers, ", "))
	}
}
