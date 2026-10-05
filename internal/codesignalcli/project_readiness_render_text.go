package codesignalcli

import (
	"fmt"
	"strings"
)

func renderReadinessNextActionLine(b *strings.Builder, action ReadinessNextAction) {
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
