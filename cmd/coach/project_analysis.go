package main

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// preparedProject is what loading a policy yields: the analysis to run, or the
// diagnostics that explain why project analysis is off. Returning it by value
// leaves "no analysis" as an empty field instead of a nil pointer result.
type preparedProject struct {
	analysis    *codesignalcli.ProjectAnalysis
	diagnostics []codesignal.Diagnostic
	// language is the language the policy was prepared for, empty where no policy was read.
	language string
}

func prepareProjectAnalysis(dir, revision string, named policyref.Ref, language projectLanguageFlag, ports scanPorts) (preparedProject, error) {
	if named.JudgedOnDisk() {
		disk, err := judgeNamedOnDisk(dir, revision, named, ports)
		if err != nil {
			return preparedProject{}, withReadinessForLanguage(err, dir, revision, named, language.value)
		}
		return loadPreparedProject(dir, revision, named, language, nil, disk, ports)
	}
	selection, err := ports.discover(dir, revision)
	if err != nil {
		return preparedProject{}, err
	}
	disk, err := judgeDiscoveredOnDisk(dir, revision, selection, ports)
	if err != nil {
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, policyref.Ref{}, language.value)
	}
	if selection.Fatal != nil && refusesWhateverDiskHolds(selection.Fatal) {
		// Path stays empty: Load rejected the canonical file. The TypeScript
		// wrap is what prints the gaps that rejection would otherwise hide.
		return preparedProject{}, withReadinessForLanguage(selection.Fatal, dir, revision, policyref.Ref{}, language.value)
	}
	return loadPreparedProject(dir, revision, discoveredSource(selection), language, projectSelectionDiagnostics(selection), disk, ports)
}

// refusesWhateverDiskHolds is whether a failure to read the committed canonical policy stands whatever the working tree
// holds. A refusal of the committed copy never decides a scan, since the policy on disk is used or none is; an
// operational failure is no refusal of the copy, and stands.
func refusesWhateverDiskHolds(fatal error) bool {
	var refusal *projectconfig.ConfigError
	return !errors.As(fatal, &refusal)
}

// discoveredSource is the policy the revision selects. A committed canonical policy that is not usable still is the
// policy it selects, so a scan that does not use it can say that it is absent.
func discoveredSource(selection projectconfig.Selection) policyref.Ref {
	if selection.Fatal != nil {
		return policyref.Discovered(projectconfig.CanonicalPath)
	}
	return policyref.Discovered(selection.Path)
}

func withReadinessForLanguage(err error, dir, revision string, policy policyref.Ref, language string) error {
	if language != "typescript" {
		return err
	}
	return tssetup.WrapProjectConfigErrorWithReadiness(err, dir, revision, policy)
}

func backendUnavailableDiagnostic(policyPath, message string) codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Kind:    "project_backend_unavailable",
		Path:    policyPath,
		Message: message,
	}
}

func appendDiagnostic(diags []codesignal.Diagnostic, diag codesignal.Diagnostic) []codesignal.Diagnostic {
	out := append([]codesignal.Diagnostic{}, diags...)
	return append(out, diag)
}
