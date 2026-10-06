package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --project-config with the real Go project-language backend", func() {
	When("comparing JSON and text output for the same baseline layer-violation scenario", func() {
		It("presents the same structured evidence in text as JSON, and legacy (no config) text stays schema-1", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			By("committing a second importer of pkg/db so the violation group has two sites and RelatedLocations is non-empty")
			commitFile(repo, "pkg/handlers/other.go", "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Other() string {\n\treturn db.Name\n}\n")
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			jsonStdout, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
			report := decodeCoachReport(jsonStdout)
			Expect(report.ProjectChanges).To(HaveLen(1))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)

			Expect(text).To(ContainSubstring("Project findings:"))
			Expect(text).To(ContainSubstring("semantic_key: " + report.ProjectChanges[0].SemanticKey))
			Expect(text).To(ContainSubstring("rule_id: architecture.layer_violation"))
			Expect(text).To(ContainSubstring("path: pkg/handlers/handlers.go"))
			Expect(text).To(ContainSubstring("lifecycle: baseline"))
			Expect(text).To(ContainSubstring("machine_evidence.importer: pkg/handlers"))
			Expect(text).To(ContainSubstring("machine_evidence.importee: pkg/db"))
			Expect(text).To(ContainSubstring("Project summary: active=1"))
			Expect(text).To(ContainSubstring("Project coverage: phase=go_model_build, complete=true"))

			By("asserting signals[] carries the same structured machine_evidence text shows, so a consumer reading only signals gets full parity")
			Expect(report.Signals).To(HaveLen(1))
			sig := report.Signals[0]
			Expect(sig.MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers",
				"importee":   "pkg/db",
				"layer_from": "handlers",
				"layer_to":   "db",
				"rule":       "handlers->db",
			}))
			for key, value := range sig.MachineEvidence {
				Expect(text).To(ContainSubstring("machine_evidence." + key + ": " + value))
			}
			Expect(sig.RelatedLocations).NotTo(BeEmpty())
			Expect(sig.RelatedLocations).To(Equal(report.ProjectChanges[0].RelatedLocations))

			By("asserting text shows the same related location JSON RelatedLocations carries, not only the primary anchor")
			for _, location := range sig.RelatedLocations {
				Expect(text).To(ContainSubstring(fmt.Sprintf("related: %s:%d", location.Path, location.Location.StartRow+1)))
			}

			legacyStdout, legacyStderr, legacyExit := runCoachCodesignalBaselineRaw(repo)
			Expect(legacyExit).To(Equal(0), "stderr: %s", legacyStderr)
			legacyText := string(legacyStdout)
			Expect(legacyText).NotTo(ContainSubstring("Project findings:"))
			Expect(legacyText).NotTo(ContainSubstring("Project coverage:"))
		})
	})

	When("a baseline scan produces both an architecture layer-violation finding and a low-severity structural finding in the same lifecycle group", func() {
		It("orders the architecture finding ahead of the low-severity structural finding in signals[]", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			By("committing the structural finding so it naturally lands first in the pre-sort signals slice (Build appends file-local signals before project signals) -- sortSignals's sort.SliceStable would let that incidental order pass this test for the wrong reason unless the comparator itself is what decides the final order")
			commitFile(repo, "pkg/model/model.go", modelFileWithTwoConstructors)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)

			architectureIndex := firstSignalIndex(report.Signals, "architecture.layer_violation", "architecture.layer_bypass")
			structuralIndex := firstSignalIndex(report.Signals, "structure.constructor_density")

			Expect(architectureIndex).To(BeNumerically(">=", 0), "expected an architecture.layer_violation or architecture.layer_bypass signal in signals[]: %s", stdout)
			Expect(structuralIndex).To(BeNumerically(">=", 0), "expected a structure.constructor_density signal in signals[]: %s", stdout)

			Expect(report.Signals[architectureIndex].Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(report.Signals[architectureIndex].Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(report.Signals[structuralIndex].Severity).To(Equal(codesignal.Severity("low")))
			Expect(report.Signals[architectureIndex].Lifecycle).To(Equal(report.Signals[structuralIndex].Lifecycle), "both findings must belong to the same lifecycle group for this to test the severity comparator rather than group ordering")

			Expect(architectureIndex).To(BeNumerically("<", structuralIndex), "an advisory-severity, high-confidence architecture finding must outrank a low-severity structural finding in signals[] order (issue #259)")
		})
	})
})
