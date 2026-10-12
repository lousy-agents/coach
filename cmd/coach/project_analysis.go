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

func prepareProjectAnalysis(dir, revision string, projectConfigSet bool, configPath, language string) (*codesignalcli.ProjectAnalysis, *codesignal.Diagnostic, error) {
	if !projectConfigSet {
		return nil, nil, nil
	}
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
		return nil, &codesignal.Diagnostic{
			Kind:    "project_backend_unavailable",
			Path:    configPath,
			Message: backendErr.Message,
		}, nil
	}
	backend := lookupProjectBackend(language)
	if backend == nil {
		return nil, &codesignal.Diagnostic{
			Kind:    "project_backend_unavailable",
			Path:    configPath,
			Message: fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language),
		}, nil
	}
	return &codesignalcli.ProjectAnalysis{
		ConfigPath:   configPath,
		Language:     language,
		Config:       append(json.RawMessage(nil), config...),
		ConfigDigest: projectconfig.Digest(config),
		Backend:      backend,
	}, nil, nil
}
