package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_noFindingsVerdictAcceptanceTest_100() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_noFindingsVerdictAcceptanceTest_rendersDifferentVerdictTextForTheTwoRunsNotMerel_133() {
	cleanRepo := newTempGitRepo()
	commitFile(cleanRepo, "go.mod", goModuleFile)
	commitFile(cleanRepo, "pkg/db/db.go", dbPackageFile)
	commitFile(cleanRepo, "pkg/handlers/handlers.go", handlersWithoutImport)
	commitFile(cleanRepo, "project.json", goLayerPolicyConfigJSON)

	By("using db.go/handlersImportingDB: project_go_backend_acceptance_test.go's unambiguous-edge spec already proves this exact fixture reports with a normal-sized go.mod, so the oversized go.mod below suppresses a real finding, not an absent one")
	incompleteRepo := newTempGitRepo()
	commitFile(incompleteRepo, "go.mod", oversizedGoModuleFile())
	commitFile(incompleteRepo, "pkg/db/db.go", dbPackageFile)
	commitFile(incompleteRepo, "pkg/handlers/handlers.go", handlersImportingDB)
	commitFile(incompleteRepo, "project.json", goLayerPolicyConfigJSON)

	_, incompleteReport := expectIncompleteVerdictDiscrimination(cleanRepo, incompleteRepo,
		"discoverGoProject reports the unreadable go.mod through Coverage/Diagnostics rather than a hard error; exit status alone cannot discriminate these two runs",
		"the unreadable go.mod",
		"No active CodeSignal findings.\n")

	foundRootUnavailable := false
	for _, diag := range incompleteReport.ProjectCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagRootUnavailable {
			foundRootUnavailable = true
		}
	}
	Expect(foundRootUnavailable).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagRootUnavailable, incompleteReport.ProjectCoverage.Diagnostics)
}
