package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func body_ruleLayerBypassAcceptanceTest_leavesPrimaryAnchorEmptyRatherThanFabricatingOne_172() {
	witness := domain.LayerBypassWitness{
		ID:            "bypass:service:example.com/app/handlers.Handler->(*database/sql.DB).Query@" + domain.LayerBypassAlgorithm,
		Source:        "example.com/app/handlers.Handler",
		Sink:          "(*database/sql.DB).Query",
		RequiredLayer: "service",
		Path: []domain.LayerBypassStep{
			{NodeID: "example.com/app/handlers.Handler"},
			{NodeID: "(*database/sql.DB).Query"},
		},
		Confidence:       domain.LayerBypassConfidenceHigh,
		AlgorithmVersion: domain.LayerBypassAlgorithm,
	}

	changes, diagnostics := codesignal.EvaluateGoLayerBypass(layerBypassResult(witness), "1", "backend-1", "digest-1")
	Expect(diagnostics).To(BeEmpty())
	Expect(changes).To(HaveLen(1))
	Expect(changes[0].PrimaryAnchor).To(Equal(codesignal.ProjectLocation{}))

	report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
		ProjectChanges:  changes,
		ProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
	})

	Expect(report.ProjectChanges).To(BeEmpty())
	found := false
	for _, d := range report.Diagnostics {
		if d.Kind == "project_observation_missing_primary_path" {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "expected a project_observation_missing_primary_path diagnostic, got %+v", report.Diagnostics)
}

func body_ruleLayerBypassAcceptanceTest_587(source, sink, requiredLayer string, path []string) domain.LayerBypassWitness {
	steps := make([]domain.LayerBypassStep, len(path))
	for i, nodeID := range path {
		steps[i] = domain.LayerBypassStep{NodeID: nodeID}
	}
	if len(steps) > 0 {
		steps[0].Path = layerBypassSourcePath
	}
	return domain.LayerBypassWitness{
		ID:               "bypass:" + requiredLayer + ":" + source + "->" + sink + "@" + domain.TSLayerBypassAlgorithm,
		Source:           source,
		Sink:             sink,
		RequiredLayer:    requiredLayer,
		Path:             steps,
		Confidence:       domain.LayerBypassConfidenceHigh,
		AlgorithmVersion: domain.TSLayerBypassAlgorithm,
	}
}
