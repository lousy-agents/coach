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
}

func prepareProjectAnalysis(dir, revision string, named policyref.Ref, language string, ports scanPorts) (preparedProject, error) {
	if named.JudgedOnDisk() {
		disk, err := judgeNamedOnDisk(dir, revision, named, ports)
		if err != nil {
			return preparedProject{}, withReadinessForLanguage(err, dir, revision, named, language)
		}
		return loadPreparedProject(dir, revision, named, language, nil, disk, ports)
	}
	selection, err := ports.discover(dir, revision)
	if err != nil {
		return preparedProject{}, err
	}
	disk, err := judgeDiscoveredOnDisk(dir, revision, selection, ports)
	if err != nil {
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, policyref.Ref{}, language)
	}
	if selection.Fatal != nil && !supersededByDisk(selection.Fatal, disk) {
		// Path stays empty: Load rejected the canonical file. The TypeScript
		// wrap is what prints the gaps that rejection would otherwise hide.
		return preparedProject{}, withReadinessForLanguage(selection.Fatal, dir, revision, policyref.Ref{}, language)
	}
	return loadPreparedProject(dir, revision, policyref.Discovered(selection.Path), language, projectSelectionDiagnostics(selection), disk, ports)
}

// supersededByDisk is whether a refusal of the committed canonical policy gives way to a canonical policy that is usable
// on disk, which the scan uses whatever the revision holds there.
func supersededByDisk(fatal error, disk projectconfig.DiskSelection) bool {
	var refusal *projectconfig.ConfigError
	return disk.Path == projectconfig.CanonicalPath && errors.As(fatal, &refusal)
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
