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
	renderReadinessCheckLine(&b, "compiler", result.Checks.Compiler)
	renderReadinessCheckLine(&b, "package_manager", result.Checks.PackageManager)

	if len(result.Gaps) > 0 {
		b.WriteString("\nGaps:\n")
		for _, gap := range result.Gaps {
			fmt.Fprintf(&b, "  %s\n", gap.Code)
		}
	}

	if len(result.Warnings) > 0 {
		b.WriteString("\nWarnings:\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&b, "  %s (found_major=%d tested_major=%d floor_major=%d)\n", warning.Code, warning.FoundMajor, warning.TestedMajor, warning.FloorMajor)
		}
	}

	if len(result.NextActions) > 0 {
		b.WriteString("\nNext actions:\n")
		for _, action := range result.NextActions {
			fmt.Fprintf(&b, "  %s\n", action.Kind)
		}
	}

	if result.DirtyWorktree.RelevantChanges {
		b.WriteString("\nWarning: uncommitted or untracked changes exist under paths relevant to this result. ")
		b.WriteString("These paths are not part of the analyzed revision and had no effect on the checks above:\n")
		for _, changedPath := range result.DirtyWorktree.Paths {
			fmt.Fprintf(&b, "  %s\n", changedPath)
		}
	}

	return b.String()
}

func renderReadinessCheckLine(b *strings.Builder, name string, check ReadinessCheck) {
	fmt.Fprintf(b, "  %s: %s", name, check.State)
	if check.Code != "" {
		fmt.Fprintf(b, " (%s)", check.Code)
	}
	if check.Version != "" {
		fmt.Fprintf(b, " version=%s", check.Version)
	}
	if check.ExpectedVersion != "" {
		fmt.Fprintf(b, " expected_version=%s", check.ExpectedVersion)
	}
	if check.FoundVersion != "" {
		fmt.Fprintf(b, " found_version=%s", check.FoundVersion)
	}
	b.WriteString("\n")
}
