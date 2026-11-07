package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

const layerlessPolicyAuthoringPrompt = "%s is valid but declares no layers, so layer rules have nothing to check. Start guided policy authoring? [y/N] "

func layerlessDiscoveredPolicy(report *codesignal.Report, f codesignalFlags) bool {
	if f.projectLanguage != "typescript" || f.namedPolicy.Explicit || report.ProjectScope == nil {
		return false
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind == "project_config_deprecated" {
			return false
		}
	}
	head := report.ProjectScope.Head
	return len(head.MatchedLayers)+len(head.UnmatchedLayers) == 0
}

// offerAuthoringForLayerlessDiscoveredPolicy runs after the report has been
// rendered, so stdout stays one parseable report. The candidate is printed
// on stderr for the same reason, and the scan's exit code is left alone.
func offerAuthoringForLayerlessDiscoveredPolicy(dir string, f codesignalFlags, report *codesignal.Report, stderr *os.File, ports scanPorts) {
	if !layerlessDiscoveredPolicy(report, f) || nonInteractiveRequested(f) || !terminal.HasControllingTerminal(os.Stdin) {
		return
	}
	fmt.Fprintf(stderr, layerlessPolicyAuthoringPrompt, projectconfig.CanonicalPath)
	if !answeredYes(os.Stdin) {
		return
	}
	if authorProjectConfigTypeScript(dir, f, os.Stdin, stderr, stderr, ports) != 0 {
		return
	}
	fmt.Fprintf(stderr, "coach codesignal: the approved TypeScript project-config candidate was printed above and no file was created; review it, save it as %q replacing the current file, commit it, then rerun this scan once it is committed.\n", projectconfig.CanonicalPath)
}
