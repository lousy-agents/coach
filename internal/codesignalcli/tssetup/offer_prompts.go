package tssetup

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

// promptForSetupConfirmation is the single-use explicit confirmation gate
// for a project_package setup command: only the exact token "confirm"
// (case-insensitive) proceeds, read exactly once from reader -- there is no
// retry, mirroring promptForMiseInstallConfirmation's own gate.
func promptForSetupConfirmation(out io.Writer, reader *bufio.Reader) bool {
	fmt.Fprintln(out, "Type 'confirm' to run this setup command now, or anything else to cancel without making any change:")
	fmt.Fprint(out, "> ")
	answer, _ := prompt.ReadLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "confirm")
}

// promptForCompilerSetupChoice requires the user to type one offered
// choice's exact kind name, or "cancel". There is no numbered/default
// selection: an unrecognized or blank answer cancels rather than falling
// back to any choice, mirroring promptForMiseSetupChoice's own contract
// (project_ts_compiler_mise_install.go).
func promptForCompilerSetupChoice(out io.Writer, reader *bufio.Reader, choices []Choice) (ChoiceKind, bool) {
	fmt.Fprintln(out, "TypeScript compiler setup: the following choices are offered to resolve the failing compiler check:")
	for _, c := range choices {
		fmt.Fprintf(out, "  - %s%s\n", c.Kind, setupChoiceScopeClause(c.Kind))
	}
	fmt.Fprintln(out, "Type the exact choice name to select it, or 'cancel' to cancel without making any change. There is no default: an unrecognized or blank answer cancels.")
	fmt.Fprint(out, "> ")
	answer, unreadable := prompt.ReadLine(reader)
	if unreadable {
		return "", true
	}
	answer = strings.TrimSpace(answer)
	for _, c := range choices {
		if answer != string(c.Kind) {
			continue
		}
		if c.Kind == ChoiceCancel {
			return "", true
		}
		return c.Kind, false
	}
	return "", true
}

func printSetupPreview(out io.Writer, preview Preview) {
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

// setupChoiceScopeClause names what each choice would touch, at the moment
// the customer picks one. The full AC-SET-2 preview still precedes the
// confirmation, but it arrives only after a selection, so without this the
// selection itself is made from bare machine identifiers -- and
// project_mise and global_mise differ in exactly the property a customer
// would want to know before choosing between them.
func setupChoiceScopeClause(kind ChoiceKind) string {
	switch kind {
	case ChoiceProjectPackage:
		return " (runs this project's own package manager in the selected manifest context)"
	case ChoiceProjectMise:
		return " (installs the version this repository's mise configuration pins, into mise's shared tool store)"
	case ChoiceGlobalMise:
		return " (installs the version your global mise configuration pins, into mise's shared tool store)"
	case ChoiceCancel:
		return " (change nothing and stop this scan)"
	default:
		return ""
	}
}
