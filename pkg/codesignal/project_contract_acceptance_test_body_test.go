package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

func body_projectContractAcceptanceTest_changesIdentityWhenRuleBackendOrConfigurationIde_20() {
	base := projectChange("cycle:pkg/a<->pkg/b", "project.import_cycle")
	base.RuleVersion = "1"
	base.BackendVersion = "go-backend-v1"
	base.AlgorithmVersion = "cycle-v1"
	base.ConfigDigest = "config-a"

	variants := []codesignal.ProjectChange{base}
	for _, mutate := range []func(*codesignal.ProjectChange){
		func(change *codesignal.ProjectChange) { change.RuleVersion = "2" },
		func(change *codesignal.ProjectChange) { change.BackendVersion = "go-backend-v2" },
		func(change *codesignal.ProjectChange) { change.AlgorithmVersion = "cycle-v2" },
		func(change *codesignal.ProjectChange) { change.ConfigDigest = "config-b" },
	} {
		variant := base
		mutate(&variant)
		variants = append(variants, variant)
	}

	identities := make([]string, 0, len(variants))
	for _, change := range variants {
		report := build(codesignal.Options{ProjectEnabled: true}, codesignal.Input{
			ProjectChanges:  []codesignal.ProjectChange{change},
			ProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
		})
		identities = append(identities, report.ProjectChanges[0].ID+"/"+report.ProjectChanges[0].Fingerprint)
	}
	Expect(identities[1:]).NotTo(ContainElement(identities[0]))
}
