package main

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_projectGoBackendAcceptanceTest_emitsExactlyOneArchitectureLayerBypassProjectCha_548() {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/service/service.go", servicePackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantAndBypass)
	commitFile(repo, "project.json", goLayerBypassPolicyConfigJSON)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	Expect(stderr).To(BeEmpty())

	report := decodeCoachReport(stdout)
	Expect(report.ProjectChanges).To(HaveLen(1), "expected exactly one deterministic layer_bypass witness, got %+v", report.ProjectChanges)
	change := report.ProjectChanges[0]
	expectLayerBypassChange(change)
	Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))

	Expect(report.ProjectSummary).NotTo(BeNil())
	Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))
	Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))

	var found bool
	for _, sig := range report.Signals {
		if sig.RuleID == "architecture.layer_bypass" {
			found = true
			Expect(sig.Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(sig.Confidence).To(Equal(codesignal.Confidence("high")))
		}
	}
	Expect(found).To(BeTrue(), "expected an architecture.layer_bypass entry in signals[]: %s", stdout)
}

func body_projectGoBackendAcceptanceTest_emitsExactlyOneArchitectureLayerBypassProjectCha_582() {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/service/service.go", servicePackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantOnly)
	baseSHA := commitFile(repo, "project.json", goLayerBypassPolicyConfigJSON)
	commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantAndBypass)

	stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	Expect(stderr).To(BeEmpty())

	report := decodeCoachReport(stdout)
	Expect(report.ProjectChanges).To(HaveLen(1), "expected exactly one deterministic layer_bypass witness, got %+v", report.ProjectChanges)
	change := report.ProjectChanges[0]
	expectLayerBypassChange(change)
	Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
	Expect(change.Changed).To(BeTrue())
	Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))

	var found bool
	for _, sig := range report.Signals {
		if sig.RuleID == "architecture.layer_bypass" {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "expected an architecture.layer_bypass entry in signals[]: %s", stdout)
}
