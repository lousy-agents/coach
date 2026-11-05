package main

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// policySource is the policy a scan reads: the path, and whether the user named
// it with --project-config or the scan discovered it. The two are read
// differently, so they travel together.
type policySource struct {
	path     string
	explicit bool
}

// preparedProject is what loading a policy yields: the analysis to run, or the
// diagnostics that explain why project analysis is off. Returning it by value
// leaves "no analysis" as an empty field instead of a nil pointer result.
type preparedProject struct {
	analysis    *codesignalcli.ProjectAnalysis
	diagnostics []codesignal.Diagnostic
}

// namedPolicy is the policy the user named with --project-config, or the zero
// policySource when the scan is to discover one.
func (f codesignalFlags) namedPolicy() policySource {
	if !f.projectConfigSet {
		return policySource{}
	}
	return policySource{path: f.projectConfig, explicit: true}
}

func prepareProjectAnalysis(dir, revision string, named policySource, language string, suppressUncommittedCanonical bool, ports scanPorts) (preparedProject, error) {
	if named.explicit {
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
			return preparedProject{}, tssetup.WrapProjectConfigErrorWithReadiness(selection.Fatal, dir, revision, "")
		}
		return preparedProject{}, selection.Fatal
	}
	diags := projectSelectionDiagnostics(selection, revision, suppressUncommittedCanonical)
	if selection.Path == "" {
		return preparedProject{diagnostics: diags}, nil
	}
	return loadPreparedProject(dir, revision, policySource{path: selection.Path}, language, diags, ports)
}

func loadPreparedProject(dir, revision string, source policySource, language string, diags []codesignal.Diagnostic, ports scanPorts) (preparedProject, error) {
	config, err := ports.loader(source.explicit)(dir, revision, source.path)
	if err != nil {
		namedPath := ""
		if source.explicit {
			namedPath = source.path
		}
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, namedPath, language)
	}
	backend, unavailable, err := selectProjectBackend(ports, language)
	if err != nil {
		return preparedProject{}, err
	}
	if backend == nil {
		return preparedProject{diagnostics: appendDiagnostic(diags, backendUnavailableDiagnostic(source.path, unavailable))}, nil
	}
	return preparedProject{
		analysis: &codesignalcli.ProjectAnalysis{
			ConfigPath:     source.path,
			ConfigExplicit: source.explicit,
			Language:       language,
			Config:         append(json.RawMessage(nil), config...),
			ConfigDigest:   projectconfig.Digest(config),
			Backend:        backend,
		},
		diagnostics: diags,
	}, nil
}

func withReadinessForLanguage(err error, dir, revision, configPath, language string) error {
	if language != "typescript" {
		return err
	}
	return tssetup.WrapProjectConfigErrorWithReadiness(err, dir, revision, configPath)
}

func backendUnavailableDiagnostic(configPath, message string) codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Kind:    "project_backend_unavailable",
		Path:    configPath,
		Message: message,
	}
}

func appendDiagnostic(diags []codesignal.Diagnostic, diag codesignal.Diagnostic) []codesignal.Diagnostic {
	out := append([]codesignal.Diagnostic{}, diags...)
	return append(out, diag)
}
