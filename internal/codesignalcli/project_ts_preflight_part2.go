package codesignalcli

import (
	"bufio"

	"fmt"
	"io"

	"strings"
)

// promptForCompilerSetupChoice requires the user to type one offered
// choice's exact kind name, or "cancel". There is no numbered/default
// selection: an unrecognized or blank answer cancels rather than falling
// back to any choice, mirroring promptForMiseSetupChoice's own contract
// (project_ts_compiler_mise_install.go).
func promptForCompilerSetupChoice(out io.Writer, reader *bufio.Reader, choices []SetupChoice) (SetupChoiceKind, bool) {
	fmt.Fprintln(out, "TypeScript compiler setup: the following choices are offered to resolve the failing compiler check:")
	for _, c := range choices {
		fmt.Fprintf(out, "  - %s%s\n", c.Kind, setupChoiceScopeClause(c.Kind))
	}
	fmt.Fprintln(out, "Type the exact choice name to select it, or 'cancel' to cancel without making any change. There is no default: an unrecognized or blank answer cancels.")
	fmt.Fprint(out, "> ")
	answer, unreadable := readLine(reader)
	if unreadable {
		return "", true
	}
	answer = strings.TrimSpace(answer)
	for _, c := range choices {
		if answer != string(c.Kind) {
			continue
		}
		if c.Kind == SetupChoiceCancel {
			return "", true
		}
		return c.Kind, false
	}
	return "", true
}

// withProjectPackageResolution re-decides the project_package entry against
// the one fact AvailableSetupChoices cannot see: which manifest context the
// install would actually run in. AvailableSetupChoices decides purely from a
// readiness snapshot and has no dir/revision to resolve a working directory
// with, so this is where an offer that cannot be previewed honestly is
// converted into a withheld entry with its reason.
func withProjectPackageResolution(menu SetupChoiceMenu, dir, revision, configPath string) (resolved SetupChoiceMenu, workingDirectory, withheldReason string) {
	if !menuOffersChoice(menu, SetupChoiceProjectPackage) {
		return menu, "", ""
	}
	workingDirectory, withheldReason = projectPackageWorkingDirectory(dir, revision, configPath)
	if withheldReason == "" {
		return menu, workingDirectory, ""
	}
	remaining := make([]SetupChoice, 0, len(menu.Choices))
	for _, choice := range menu.Choices {
		if choice.Kind == SetupChoiceProjectPackage {
			continue
		}
		remaining = append(remaining, choice)
	}
	menu.Choices = remaining
	menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: withheldReason})
	return menu, "", withheldReason
}
func printSetupPreview(out io.Writer, preview SetupPreview) {
	fmt.Fprintln(out, "Before this setup command runs, here is exactly what it will do:")
	fmt.Fprintf(out, "  Executable: %s\n", preview.Executable)
	fmt.Fprintf(out, "  Arguments: %s\n", strings.Join(preview.Args, " "))
	fmt.Fprintf(out, "  Working directory: %s\n", preview.WorkingDirectory)
	fmt.Fprintf(out, "  Expected changes: %s\n", preview.ExpectedChanges)
	fmt.Fprintf(out, "  Network use: %s\n", preview.NetworkDisclosure)
	fmt.Fprintf(out, "  Lifecycle-script policy: %s\n", preview.ScriptSuppressionPolicy)
	if preview.PinDisclosure != "" {
		fmt.Fprintf(out, "  Pin disclosure: %s\n", preview.PinDisclosure)
	}
	fmt.Fprintf(out, "  Timeout: %s\n", preview.Timeout)
}
