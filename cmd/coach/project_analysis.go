package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

var (
	loadProjectConfig     = projectconfig.Load
	discoverProjectConfig = projectconfig.Discover
	resolveProjectBackend = codesignalcli.ResolveProjectBackend
	lookupProjectBackend  = func(language string) codesignalcli.ProjectBackend {
		switch language {
		case "go":
			return codesignalcli.NewGoProjectBackend()
		case "typescript":
			return codesignalcli.NewTSProjectBackend()
		default:
			return nil
		}
	}
)

func prepareProjectAnalysis(dir, revision string, projectConfigSet bool, configPath, language string, suppressUncommittedCanonical bool) (*codesignalcli.ProjectAnalysis, []codesignal.Diagnostic, error) {
	if projectConfigSet {
		return loadPreparedProject(dir, revision, configPath, language, nil)
	}
	selection, err := discoverProjectConfig(dir, revision)
	if err != nil {
		return nil, nil, err
	}
	if selection.Fatal != nil {
		// Path stays empty: Load rejected the canonical file. The TypeScript
		// wrap is what prints the gaps that rejection would otherwise hide.
		if language == "typescript" {
			return nil, nil, tssetup.WrapProjectConfigErrorWithReadiness(selection.Fatal, dir, revision, projectconfig.CanonicalPath)
		}
		return nil, nil, selection.Fatal
	}
	diags := projectSelectionDiagnostics(selection, revision, suppressUncommittedCanonical)
	if selection.Path == "" {
		return nil, diags, nil
	}
	return loadPreparedProject(dir, revision, selection.Path, language, diags)
}

func loadPreparedProject(dir, revision, configPath, language string, diags []codesignal.Diagnostic) (*codesignalcli.ProjectAnalysis, []codesignal.Diagnostic, error) {
	config, err := loadProjectConfig(dir, revision, configPath)
	if err != nil {
		if language == "typescript" {
			err = tssetup.WrapProjectConfigErrorWithReadiness(err, dir, revision, configPath)
		}
		return nil, nil, err
	}
	if err := resolveProjectBackend(language); err != nil {
		var backendErr *codesignalcli.ProjectBackendUnavailableError
		if !errors.As(err, &backendErr) {
			return nil, nil, err
		}
		return nil, appendDiagnostic(diags, backendUnavailableDiagnostic(configPath, backendErr.Message)), nil
	}
	backend := lookupProjectBackend(language)
	if backend == nil {
		return nil, appendDiagnostic(diags, backendUnavailableDiagnostic(configPath, fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language))), nil
	}
	return &codesignalcli.ProjectAnalysis{
		ConfigPath:   configPath,
		Language:     language,
		Config:       append(json.RawMessage(nil), config...),
		ConfigDigest: projectconfig.Digest(config),
		Backend:      backend,
	}, diags, nil
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
