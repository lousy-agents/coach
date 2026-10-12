package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// ReadinessJSON renders result as its canonical JSON representation
// followed by exactly one trailing newline.
func ReadinessJSON(result *projectreadiness.Result) ([]byte, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// ReadinessText renders result as deterministic, ANSI-free plain
// text. It encodes exactly the same status, gaps, and next actions as
// ReadinessJSON -- no drift between the two renderers.
func ReadinessText(result *projectreadiness.Result) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Project readiness for revision %s (language: %s)\n", result.Revision, result.Language)
	fmt.Fprintf(&b, "status: %s\n", result.Status)

	b.WriteString("\nChecks:\n")
	renderReadinessCheckLine(&b, "project_shape", result.Checks.ProjectShape)
	renderReadinessCheckLine(&b, "policy", result.Checks.Policy)
	renderReadinessCheckLine(&b, "node", result.Checks.Node)
	renderReadinessCheckLine(&b, "runtime", result.Checks.Runtime)
	renderReadinessCheckLine(&b, "compiler", result.Checks.Compiler)
	renderReadinessCheckLine(&b, "package_manager", result.Checks.PackageManager)

	renderReadinessCodeList(&b, "Gaps", gapCodes(result.Gaps))
	renderReadinessWarnings(&b, result.Warnings)
	renderReadinessNextActions(&b, result.NextActions)
	renderReadinessDirtyWorktree(&b, result.DirtyWorktree)

	return b.String()
}

func renderReadinessNextActionLine(b *strings.Builder, action projectreadiness.NextAction) {
	fmt.Fprintf(b, "  %s (executable=%t)", action.Kind, action.Executable)
	if action.RuntimeKind != "" {
		fmt.Fprintf(b, " runtime_kind=%s", action.RuntimeKind)
	}
	if action.PackageManagerKind != "" {
		fmt.Fprintf(b, " package_manager_kind=%s", action.PackageManagerKind)
	}
	if len(action.Choices) > 0 {
		fmt.Fprintf(b, " choices=%s", strings.Join(action.Choices, ","))
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

func renderReadinessCheckLine(b *strings.Builder, name string, check projectreadiness.Check) {
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
