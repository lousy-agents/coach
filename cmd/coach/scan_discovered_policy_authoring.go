package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

const layerlessPolicyAuthoringPrompt = "%s is valid but declares no layers, so layer rules have nothing to check. Start guided policy authoring? [y/N] "

// scanShouldAuthorDiscoveredProjectConfig reports whether err is a committed
// .lousy-agents/coach.json that the scan discovered on its own and that
// failed schema validation. The policy was not named with --project-config,
// so the explicit-path trigger in scanShouldAuthorProjectConfig never sees it.
func scanShouldAuthorDiscoveredProjectConfig(err error, f codesignalFlags, noInteractive bool) bool {
	if f.projectLanguage != "typescript" || f.projectConfigSet || noInteractive {
		return false
	}
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) || configErr.Kind != projectconfig.KindInvalid {
		return false
	}
	return terminal.HasControllingTerminal(os.Stdin)
}

func layerlessDiscoveredPolicy(report *codesignal.Report, f codesignalFlags) bool {
	if f.projectLanguage != "typescript" || f.projectConfigSet || report.ProjectScope == nil {
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
func offerAuthoringForLayerlessDiscoveredPolicy(dir string, f codesignalFlags, report *codesignal.Report, stderr *os.File) {
	if !layerlessDiscoveredPolicy(report, f) || nonInteractiveRequested(f) || !terminal.HasControllingTerminal(os.Stdin) {
		return
	}
	fmt.Fprintf(stderr, layerlessPolicyAuthoringPrompt, projectconfig.CanonicalPath)
	if !answeredYes(os.Stdin) {
		return
	}
	if authorProjectConfigTypeScript(dir, f, os.Stdin, stderr, stderr) != 0 {
		return
	}
	fmt.Fprintf(stderr, "coach codesignal: the approved TypeScript project-config candidate was printed above and no file was created; review it, save it as %q replacing the current file, commit it, then rerun this scan once it is committed.\n", projectconfig.CanonicalPath)
}
