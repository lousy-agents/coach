package main

import (
	"encoding/json"

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
}

func prepareProjectAnalysis(dir, revision string, named policyref.Ref, language string, suppressUncommittedCanonical bool, ports scanPorts) (preparedProject, error) {
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
	diags := projectSelectionDiagnostics(selection, revision, suppressUncommittedCanonical)
	if selection.Path == "" {
		return preparedProject{diagnostics: diags}, nil
	}
	return loadPreparedProject(dir, revision, policyref.Discovered(selection.Path), language, diags, ports)
}

func loadPreparedProject(dir, revision string, source policyref.Ref, language string, diags []codesignal.Diagnostic, ports scanPorts) (preparedProject, error) {
	config, err := ports.loader(source.JudgedOnDisk())(dir, revision, source.Path)
	if err != nil {
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, source, language)
	}
	backend, unavailable, err := selectProjectBackend(ports, language)
	if err != nil {
		return preparedProject{}, err
	}
	if backend == nil {
		return preparedProject{diagnostics: appendDiagnostic(diags, backendUnavailableDiagnostic(source.Path, unavailable))}, nil
	}
	return preparedProject{
		analysis: &codesignalcli.ProjectAnalysis{
			Policy:       source,
			Language:     language,
			Config:       append(json.RawMessage(nil), config...),
			ConfigDigest: projectconfig.Digest(config),
			Backend:      backend,
		},
		diagnostics: diags,
	}, nil
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
