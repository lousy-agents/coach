package codesignalcli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderReadinessJSON renders result as its canonical JSON representation
// followed by exactly one trailing newline.
func RenderReadinessJSON(result *ReadinessResult) ([]byte, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// RenderReadinessText renders result as deterministic, ANSI-free plain
// text. It encodes exactly the same status, gaps, and next actions as
// RenderReadinessJSON -- no drift between the two renderers.
func RenderReadinessText(result *ReadinessResult) string {
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
