package main

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// loadPreparedProject prepares the analysis of the policy the scan uses. source is the policy the revision selects, or,
// for a discovered policy that is not committed, an empty Ref: the working tree can still hold one. disk is what the
// working tree selected for it, already judged usable; a scan whose working tree selects none is file-local.
func loadPreparedProject(dir, revision string, source policyref.Ref, language projectLanguageFlag, diags []codesignal.Diagnostic, disk projectconfig.DiskSelection, ports scanPorts) (preparedProject, error) {
	evidence, err := gatherPolicyEvidence(dir, revision, source, disk, ports)
	if err != nil {
		return preparedProject{}, withReadinessForLanguage(err, dir, revision, source, language.value)
	}
	diags = withDiskSelectionDiagnostics(diags, source, disk)
	choice := choosePolicy(evidence)
	if choice.scope.Path == "" {
		return preparedProject{diagnostics: withoutOutdatedDeprecation(diags, "")}, nil
	}
	return prepareAnalysis(choice, source, language, diags, ports)
}

// gatherPolicyEvidence reads what the revision holds at the path of a policy that is usable on disk, for comparison only,
// so it refuses nothing. Where the working tree selects none, the scan uses none; only a named policy is refused.
func gatherPolicyEvidence(dir, revision string, source policyref.Ref, disk projectconfig.DiskSelection, ports scanPorts) (policyEvidence, error) {
	evidence := policyEvidence{
		headPath:         source.Path,
		disk:             disk,
		holdsConvertedAt: func(path string) bool { return projectconfig.HoldsCommittedPolicyConverted(dir, revision, path) },
	}
	if disk.Path == "" {
		return evidence, refuseExplicitPolicyMissingOnDisk(dir, revision, source, ports)
	}
	var err error
	evidence.head, err = readHeadCopy(dir, revision, source, disk.Path, ports)
	return evidence, err
}

// prepareAnalysis is the scan of the policy the choice names; a scan with no backend for the language reports why, and
// still discloses that policy. The language is the flag's if it was given, otherwise the policy's.
func prepareAnalysis(choice policyChoice, source policyref.Ref, flag projectLanguageFlag, diags []codesignal.Diagnostic, ports scanPorts) (preparedProject, error) {
	config, err := projectconfig.Parse(choice.content)
	if err != nil {
		return preparedProject{}, err
	}
	language := chooseProjectLanguage(flag, config.Language)
	if override, overrides := language.overrideDiagnostic(choice.scope.Path); overrides {
		diags = appendDiagnostic(diags, override)
	}
	backend, unavailable, err := selectProjectBackend(ports, language.scope.Language)
	if err != nil {
		return preparedProject{}, err
	}
	if backend == nil {
		kept := withoutOutdatedDeprecation(diags, choice.scope.Path)
		return preparedProject{diagnostics: appendDiagnostic(kept, backendUnavailableDiagnostic(choice.scope.Path, unavailable)), language: language.scope.Language}, nil
	}
	return analysisOf(choice, source, language, backend, diags), nil
}

func analysisOf(choice policyChoice, source policyref.Ref, language chosenProjectLanguage, backend codesignalcli.ProjectBackend, diags []codesignal.Diagnostic) preparedProject {
	return preparedProject{
		analysis: &codesignalcli.ProjectAnalysis{
			Policy:        policyref.Ref{Path: choice.scope.Path, Explicit: source.Explicit},
			Language:      language.scope.Language,
			LanguageScope: language.scope,
			Config:        append(json.RawMessage(nil), choice.content...),
			ConfigDigest:  choice.scope.Digest,
			PolicyScope:   choice.scope,
			Backend:       backend,
		},
		diagnostics: withoutOutdatedDeprecation(diags, choice.scope.Path),
		language:    language.scope.Language,
	}
}
