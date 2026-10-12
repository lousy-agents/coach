package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func renderReadinessWarningLine(b *strings.Builder, warning projectreadiness.Warning) {
	if warning.Code == projectreadiness.WarnCompilerDeclarationMismatch {
		fmt.Fprintf(b, "  %s (declared_version=%s found_version=%s declaration_origin=%s", warning.Code, warning.DeclaredVersion, warning.FoundVersion, warning.DeclarationOrigin)
		if warning.Root != "" {
			fmt.Fprintf(b, " root=%s", warning.Root)
		}
		b.WriteString(")\n")
		return
	}
}

func renderReadinessNextActions(b *strings.Builder, actions []projectreadiness.NextAction) {
	if len(actions) == 0 {
		return
	}
	b.WriteString("\nNext actions:\n")
	for _, action := range actions {
		renderReadinessNextActionLine(b, action)
	}
}

func renderReadinessCodeList(b *strings.Builder, heading string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", heading)
	for _, value := range values {
		fmt.Fprintf(b, "  %s\n", value)
	}
}

func renderReadinessDirtyWorktree(b *strings.Builder, dirty projectreadiness.DirtyWorktree) {
	if !dirty.RelevantChanges {
		return
	}
	b.WriteString("\nWarning: uncommitted or untracked changes exist under paths relevant to this result. ")
	b.WriteString("These paths are not part of the analyzed revision and had no effect on the checks above:\n")
	for _, changedPath := range dirty.Paths {
		fmt.Fprintf(b, "  %s\n", changedPath)
	}
}

func renderReadinessWarnings(b *strings.Builder, warnings []projectreadiness.Warning) {
	if len(warnings) == 0 {
		return
	}
	b.WriteString("\nWarnings:\n")
	for _, warning := range warnings {
		renderReadinessWarningLine(b, warning)
	}
}
