package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func verdictLine(text string) string {
	idx := strings.Index(text, "No active CodeSignal findings")
	ExpectWithOffset(1, idx).To(BeNumerically(">=", 0), "expected a rendered verdict line in: %s", text)
	rest := text[idx:]
	end := strings.IndexByte(rest, '\n')
	ExpectWithOffset(1, end).To(BeNumerically(">=", 0))
	return rest[:end+1]
}

// oversizedGoModuleFilePaddingBytes exceeds maxSnapshotFileBytes (the 32 MiB
// git-read bound in internal/codesignalcli/project_snapshot.go), so go.mod's
// read fails in pkg/projectmodel's discoverGoProject before modfile.Parse,
// setting Complete=false without registering the module. Do not shrink this:
// below the bound the read succeeds and reroutes through a different
// (DiagRootInvalid parse-failure) path where both verdicts render
// identically, silently defeating this spec. Each run costs ~60s: the
// caller's stdout-draining goroutine stops at maxStdout+1 bytes while `git
// show` is still writing this ~33 MiB blob, so cmd.Wait() only returns at
// snapshotGitTimeout, not when the oversized-read check fires.
const oversizedGoModuleFilePaddingBytes = 33 << 20

func oversizedGoModuleFile() string {
	return goModuleFile + strings.Repeat("x", oversizedGoModuleFilePaddingBytes)
}

func expectIncompleteVerdictDiscrimination(cleanRepo, incompleteRepo, exitCodeReason, cause string, extraArgs ...string) (cleanReport, incompleteReport *codesignal.Report) {
	runBoth := func(repo, format string) ([]byte, []byte, int) {
		args := append(append([]string{"--project-config", "project.json"}, extraArgs...), format)
		return runCoachCodesignalBaselineRaw(repo, args...)
	}

	cleanTextOut, cleanTextErr, cleanTextExit := runBoth(cleanRepo, "--format=text")
	ExpectWithOffset(1, cleanTextExit).To(Equal(0), "stderr: %s stdout: %s", cleanTextErr, cleanTextOut)
	ExpectWithOffset(1, cleanTextErr).To(BeEmpty())

	cleanJSONOut, cleanJSONErr, cleanJSONExit := runBoth(cleanRepo, "--format=json")
	ExpectWithOffset(1, cleanJSONExit).To(Equal(0), "stderr: %s stdout: %s", cleanJSONErr, cleanJSONOut)
	ExpectWithOffset(1, cleanJSONErr).To(BeEmpty())

	incompleteTextOut, incompleteTextErr, incompleteTextExit := runBoth(incompleteRepo, "--format=text")
	ExpectWithOffset(1, incompleteTextExit).To(Equal(0), "stderr: %s stdout: %s", incompleteTextErr, incompleteTextOut)
	ExpectWithOffset(1, incompleteTextErr).To(BeEmpty())

	incompleteJSONOut, incompleteJSONErr, incompleteJSONExit := runBoth(incompleteRepo, "--format=json")
	ExpectWithOffset(1, incompleteJSONExit).To(Equal(0), "stderr: %s stdout: %s", incompleteJSONErr, incompleteJSONOut)
	ExpectWithOffset(1, incompleteJSONErr).To(BeEmpty())

	ExpectWithOffset(1, incompleteTextExit).To(Equal(cleanTextExit), exitCodeReason)

	cleanVerdict := verdictLine(string(cleanTextOut))
	incompleteVerdict := verdictLine(string(incompleteTextOut))

	ExpectWithOffset(1, incompleteVerdict).NotTo(Equal(cleanVerdict), "a user reading either rendered report must see different verdict text for a genuinely clean run versus one with a real unreported violation")
	ExpectWithOffset(1, cleanVerdict).To(Equal("No active CodeSignal findings.\n"))
	ExpectWithOffset(1, incompleteVerdict).To(Equal("No active CodeSignal findings, but the analysis is incomplete: project analysis did not complete.\n"))

	verdictIdx := strings.Index(string(incompleteTextOut), "incomplete")
	diagnosticsIdx := strings.Index(string(incompleteTextOut), "Diagnostics:")
	ExpectWithOffset(1, verdictIdx).To(BeNumerically(">=", 0), "expected the qualified verdict clause in: %s", incompleteTextOut)
	ExpectWithOffset(1, diagnosticsIdx).To(BeNumerically(">=", 0), "expected a Diagnostics section in: %s", incompleteTextOut)
	ExpectWithOffset(1, verdictIdx).To(BeNumerically("<", diagnosticsIdx), "the qualified verdict must render before the Diagnostics block")

	cleanReport = decodeCoachReport(cleanJSONOut)
	ExpectWithOffset(1, cleanReport.ProjectChanges).To(BeEmpty())
	ExpectWithOffset(1, cleanReport.ProjectCoverage).NotTo(BeNil())
	ExpectWithOffset(1, cleanReport.ProjectCoverage.Complete).To(BeTrue())

	incompleteReport = decodeCoachReport(incompleteJSONOut)
	ExpectWithOffset(1, incompleteReport.ProjectChanges).To(BeEmpty(), cause+" must never fabricate the layer violation it never actually reported")
	ExpectWithOffset(1, incompleteReport.ProjectCoverage).NotTo(BeNil())
	ExpectWithOffset(1, incompleteReport.ProjectCoverage.Complete).To(BeFalse())

	return cleanReport, incompleteReport
}

var _ = Describe("coach codesignal rendered verdict, negative control: incomplete analysis versus a genuinely clean run", func() {
	When("one run's TS sidecar crashes over a fixture carrying a real forbidden-layer edge, and a second run's TS sidecar succeeds over a fixture with no edge at all", func() {
		It("renders different verdict text for the two runs, not merely different coverage fields", func() {
			cleanRepo := newTempGitRepo()
			installFakeTSSidecar(cleanRepo)
			commitFile(cleanRepo, "pkg/db/d.ts", tsDbFile)
			commitFile(cleanRepo, "pkg/handlers/h.ts", tsHandlersWithoutMarker)
			commitFile(cleanRepo, "project.json", goLayerPolicyConfigJSON)

			By("using tsHandlersWithMarkerAndCrash: project_ts_backend_acceptance_test.go's working-sidecar spec already proves the marker alone reports a real edge, so the crash trigger here suppresses a real finding, not an absent one")
			incompleteRepo := newTempGitRepo()
			installFakeTSSidecar(incompleteRepo)
			commitFile(incompleteRepo, "pkg/db/d.ts", tsDbFile)
			commitFile(incompleteRepo, "pkg/handlers/h.ts", tsHandlersWithMarkerAndCrash)
			commitFile(incompleteRepo, "project.json", goLayerPolicyConfigJSON)

			expectIncompleteVerdictDiscrimination(cleanRepo, incompleteRepo,
				"this backend intentionally keeps exit 0 for a degraded-but-nonfatal sidecar crash; exit status alone cannot discriminate these two runs",
				"the crashed sidecar",
				"--project-language", "typescript")
		})
	})

	When("one run's Go backend evaluates a real forbidden-layer edge to completion, and a second run's real go.mod exceeds the single-file git-read bound before that same edge is ever evaluated", func() {
		It("renders different verdict text for the two runs, not merely different coverage fields", func() {
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
				"the unreadable go.mod")

			foundRootUnavailable := false
			for _, diag := range incompleteReport.ProjectCoverage.Diagnostics {
				if diag.Code == projectmodel.DiagRootUnavailable {
					foundRootUnavailable = true
				}
			}
			Expect(foundRootUnavailable).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagRootUnavailable, incompleteReport.ProjectCoverage.Diagnostics)
		})
	})
})
