package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// preparedProject is what loading a policy yields: the analysis to run, or the
// diagnostics that explain why project analysis is off. Returning it by value
// leaves "no analysis" as an empty field instead of a nil pointer result.
type preparedProject struct {
	analysis    *codesignalcli.ProjectAnalysis
	diagnostics []codesignal.Diagnostic
}

func prepareProjectAnalysis(dir, revision string, named policyref.Ref, language string, ports scanPorts) (preparedProject, error) {
	if named.JudgedOnDisk() {
		return loadPreparedProject(dir, revision, named, language, nil, ports)
	}
	selection, err := ports.discover(dir, revision)
	if err != nil {
		return preparedProject{}, err
	}
	if selection.Fatal != nil {
		// Path stays empty: Load rejected the canonical file. The TypeScript
		// wrap is what prints the gaps that rejection would otherwise hide.
		if language == "typescript" {
			return preparedProject{}, tssetup.WrapProjectConfigErrorWithReadiness(selection.Fatal, dir, revision, policyref.Ref{})
		}
		return preparedProject{}, selection.Fatal
	}
	return loadPreparedProject(dir, revision, policyref.Discovered(selection.Path), language, projectSelectionDiagnostics(selection), ports)
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
