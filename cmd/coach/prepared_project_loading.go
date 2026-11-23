package main

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// loadPreparedProject prepares the analysis of the policy the scan uses. source is the policy the revision holds, or,
// for a discovered policy that is not committed, an empty Ref: the working tree can still hold one. disk is what the
// working tree selected for it, already judged usable.
func loadPreparedProject(dir, revision string, source policyref.Ref, language string, diags []codesignal.Diagnostic, disk projectconfig.DiskSelection, ports scanPorts) (preparedProject, error) {
	committed, err := loadCommittedPolicy(dir, revision, source, disk, ports)
	if err != nil {
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, source, language)
	}
	choice := choosePolicy(policyEvidence{
		committed:        committed,
		disk:             disk,
		holdsConvertedAt: func(path string) bool { return projectconfig.HoldsCommittedPolicyConverted(dir, revision, path) },
	})
	if choice.scope.Path == "" {
		return preparedProject{diagnostics: diags}, nil
	}
	return prepareAnalysis(choice, source, language, diags, ports)
}

// prepareAnalysis is the scan of the policy the choice names; a scan with no backend for the language reports why, and
// still discloses that policy.
func prepareAnalysis(choice policyChoice, source policyref.Ref, language string, diags []codesignal.Diagnostic, ports scanPorts) (preparedProject, error) {
	backend, unavailable, err := selectProjectBackend(ports, language)
	if err != nil {
		return preparedProject{}, err
	}
	if backend == nil {
		kept := withoutOutdatedDeprecation(diags, choice.scope.Path)
		return preparedProject{diagnostics: appendDiagnostic(kept, backendUnavailableDiagnostic(choice.scope.Path, unavailable))}, nil
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
		diagnostics: withoutOutdatedDeprecation(diags, choice.scope.Path),
	}
}
