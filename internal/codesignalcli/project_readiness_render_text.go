package codesignalcli

import (
	"fmt"
	"strings"
)

func renderReadinessCodeList(b *strings.Builder, heading string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", heading)
	for _, value := range values {
		fmt.Fprintf(b, "  %s\n", value)
	}
}

func renderReadinessWarnings(b *strings.Builder, warnings []ReadinessWarning) {
	if len(warnings) == 0 {
		return
	}
	b.WriteString("\nWarnings:\n")
	for _, warning := range warnings {
		renderReadinessWarningLine(b, warning)
	}
}

func renderReadinessWarningLine(b *strings.Builder, warning ReadinessWarning) {
	if warning.Code == WarnCompilerDeclarationMismatch {
		fmt.Fprintf(b, "  %s (declared_version=%s found_version=%s declaration_origin=%s", warning.Code, warning.DeclaredVersion, warning.FoundVersion, warning.DeclarationOrigin)
		if warning.Root != "" {
			fmt.Fprintf(b, " root=%s", warning.Root)
		}
		b.WriteString(")\n")
		return
	}
}

func renderReadinessNextActions(b *strings.Builder, actions []ReadinessNextAction) {
	if len(actions) == 0 {
		return
	}
	b.WriteString("\nNext actions:\n")
	for _, action := range actions {
		renderReadinessNextActionLine(b, action)
	}
}

func renderReadinessNextActionLine(b *strings.Builder, action ReadinessNextAction) {
	fmt.Fprintf(b, "  %s (executable=%t)", action.Kind, action.Executable)
	if action.RuntimeKind != "" {
		fmt.Fprintf(b, " runtime_kind=%s", action.RuntimeKind)
	}
	if len(action.Supported) > 0 {
		fmt.Fprintf(b, " supported=%s", strings.Join(action.Supported, ","))
	}
	if action.FoundVersion != "" {
		fmt.Fprintf(b, " found_version=%s", action.FoundVersion)
	}
	if action.Detail != "" {
		fmt.Fprintf(b, " detail=%s", action.Detail)
	}
	b.WriteString("\n")
}

func renderReadinessDirtyWorktree(b *strings.Builder, dirty ReadinessDirtyWorktree) {
	if !dirty.RelevantChanges {
		return
	}
	b.WriteString("\nWarning: uncommitted or untracked changes exist under paths relevant to this result. ")
	b.WriteString("These paths are not part of the analyzed revision and had no effect on the checks above:\n")
	for _, changedPath := range dirty.Paths {
		fmt.Fprintf(b, "  %s\n", changedPath)
	}
}

func renderReadinessCheckLine(b *strings.Builder, name string, check ReadinessCheck) {
	fmt.Fprintf(b, "  %s: %s", name, check.State)
	if check.Code != "" {
		fmt.Fprintf(b, " (%s)", check.Code)
	}
	for _, field := range readinessCheckFields(check) {
		if field.value == "" {
			continue
		}
		fmt.Fprintf(b, " %s=%s", field.label, field.value)
	}
	b.WriteString("\n")
}
