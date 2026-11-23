package main

import (
	"encoding/json"
	"sync"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// loadPreparedProject prepares the analysis of the policy the scan uses. source is the policy the revision holds, or,
// for a discovered policy that is not committed, an empty Ref: the working tree can still hold one.
func loadPreparedProject(dir, revision string, source policyref.Ref, language string, diags []codesignal.Diagnostic, ports scanPorts) (preparedProject, error) {
	disk := sync.OnceValue(func() diskPolicy { return readDiskPolicy(dir, source, ports) })
	committed, err := loadCommittedPolicy(dir, revision, source, disk, ports)
	if err != nil {
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, source, language)
	}
	if refusal := disk().refusal; refusal != nil && source.Explicit && !committed.present() {
		return preparedProject{}, withReadinessForLanguage(refusal.ExplicitError(), dir, revision, source, language)
	}
	choice := choosePolicy(policyEvidence{
		committed:        committed,
		disk:             disk(),
		holdsConvertedAt: func(path string) bool { return projectconfig.HoldsCommittedPolicyConverted(dir, revision, path) },
	})
	if !committed.present() {
		return prepareDiskOnlyProject(choice, source, language, diags, ports)
	}
	backend, unavailable, err := selectProjectBackend(ports, language)
	if err != nil {
		return preparedProject{}, err
	}
	if backend == nil {
		return preparedProject{diagnostics: appendDiagnostic(diags, backendUnavailableDiagnostic(committed.path, unavailable))}, nil
	}
	return analysisOf(choice, source, language, backend, diags), nil
}

// prepareDiskOnlyProject is the case where nothing is committed, so that no policy at all is a scan that never asks
// for a backend.
func prepareDiskOnlyProject(choice policyChoice, source policyref.Ref, language string, diags []codesignal.Diagnostic, ports scanPorts) (preparedProject, error) {
	if choice.scope.Path == "" {
		return preparedProject{diagnostics: append(diags, choice.diagnostics...)}, nil
	}
	backend, unavailable, err := selectProjectBackend(ports, language)
	if err != nil {
		return preparedProject{}, err
	}
	if backend == nil {
		return preparedProject{diagnostics: appendDiagnostic(diags, backendUnavailableDiagnostic(choice.scope.Path, unavailable))}, nil
	}
	return analysisOf(choice, source, language, backend, diags), nil
}

func analysisOf(choice policyChoice, source policyref.Ref, language string, backend codesignalcli.ProjectBackend, diags []codesignal.Diagnostic) preparedProject {
	return preparedProject{
		analysis: &codesignalcli.ProjectAnalysis{
			Policy:       policyref.Ref{Path: choice.scope.Path, Explicit: source.Explicit},
			Language:     language,
			Config:       append(json.RawMessage(nil), choice.content...),
			ConfigDigest: choice.scope.Digest,
			PolicyScope:  choice.scope,
			Backend:      backend,
		},
		diagnostics: append(withoutOutdatedDeprecation(diags, choice.scope.Path), choice.diagnostics...),
	}
}
