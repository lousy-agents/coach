package tssetup

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

// printMisePreparePreview prints the preview: executable, arguments, working
// directory, expected mise changes, network use, lifecycle-script policy,
// and timeout, describing exactly what runMiseInstallInsulated (via
// installMiseTypescriptProject/Global) actually does -- never a different,
// aspirational behavior.
func printMisePreparePreview(out io.Writer, choice, version string) {
	toolSpec := miseInstallToolSpec(version)
	fmt.Fprintln(out, "Before this setup command runs, here is exactly what it will do:")
	fmt.Fprintf(out, "  Selected mise scope: %s\n", choice)
	fmt.Fprintln(out, "  Executable: mise")
	fmt.Fprintf(out, "  Arguments: install %s; then mise where %s to locate the installed compiler\n", toolSpec, toolSpec)
	fmt.Fprintln(out, "  Working directory: a private, freshly created directory outside this repository -- mise never discovers this repository's own configuration while installing")
	fmt.Fprintf(out, "  Expected mise changes: installs TypeScript %s into mise's shared tool store (via `mise install`); does not modify this project's mise.toml or any global mise configuration file\n", version)
	fmt.Fprintf(out, "  Network use: yes -- mise downloads %s from the npm registry\n", toolSpec)
	fmt.Fprintln(out, "  Lifecycle-script policy: lifecycle scripts are suppressed for the backing npm invocation (npm_config_ignore_scripts=true); mise's own current default npm backend, and its npm.shell_out=true fallback, also suppress them independently today")
	fmt.Fprintf(out, "  Timeout: %s\n", miseInstallTimeout)
}

// promptForMiseInstallConfirmation is the single-use explicit confirmation
// gate: only the exact token "install" (case-insensitive) proceeds; there
// is no retry, mirroring promptForApproval's own gate -- the user either
// confirms what was just shown or they don't.
func promptForMiseInstallConfirmation(out io.Writer, reader *bufio.Reader) bool {
	fmt.Fprintln(out, "Type 'install' to run this setup command now, or anything else to cancel without making any change:")
	fmt.Fprint(out, "> ")
	answer, _ := prompt.ReadLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "install")
}

// promptForMiseSetupChoice requires the user to type one offered choice's
// exact name, or "cancel". There is no numbered/default selection: an
// unrecognized or blank answer cancels rather than falling back to any
// choice, including when only one is offered.
func promptForMiseSetupChoice(out io.Writer, reader *bufio.Reader, choices []string) (choice string, cancelled bool) {
	fmt.Fprintln(out, "TypeScript compiler setup: the following mise installation choices are executable and verified in this environment:")
	for _, c := range choices {
		fmt.Fprintf(out, "  - %s\n", c)
	}
	fmt.Fprintln(out, "Type the exact choice name to select it, or 'cancel' to cancel without making any change. There is no default: an unrecognized or blank answer cancels.")
	fmt.Fprint(out, "> ")
	answer, unreadable := prompt.ReadLine(reader)
	if unreadable {
		return "", true
	}
	answer = strings.TrimSpace(answer)
	for _, c := range choices {
		if answer == c {
			return c, false
		}
	}
	return "", true
}
