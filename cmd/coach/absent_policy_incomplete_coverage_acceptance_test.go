package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const failOnIncompleteCoverageFlag = "--fail-on-incomplete-coverage"

// The policy HEAD selects and the disk lacks was never read, so the scan did not apply the rules the repository
// committed. An agent that deletes the policy must not get a clean exit from the hook that exists to catch it.
var _ = Describe("a policy HEAD selects that is absent on disk counts as incomplete coverage (#168)", func() {
	for _, mode := range scopePolicyModes {
		It("#168: under "+mode.name+", a canonical policy committed at HEAD and deleted on disk exits 3 with --fail-on-incomplete-coverage and prints the report", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			expectGitStatusContains(repo, " D "+coachConfigPath)

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(3), "stdout=%s stderr=%s", stdout, stderr)
			expectDiagnosticKind(decodeCoachReport(stdout), "project_policy_absent_on_disk", coachConfigPath)
		})

		It("#168: under "+mode.name+", the same deletion exits 0 without the flag and the flag changes neither summary nor coverage", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			plain, plainStderr, plainExit := mode.run(repo, base, "--format=json")
			flagged, _, _ := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(plainExit).To(Equal(0), "stdout=%s stderr=%s", plain, plainStderr)
			plainReport, flaggedReport := decodeCoachReport(plain), decodeCoachReport(flagged)
			expectDiagnosticKind(plainReport, "project_policy_absent_on_disk", coachConfigPath)
			Expect(flaggedReport.Summary).To(Equal(plainReport.Summary))
			Expect(flaggedReport.ProjectCoverage).To(Equal(plainReport.ProjectCoverage))
		})

		It("#168: under "+mode.name+", the text report is printed before the exit 3", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			stdout, stderr, exitCode := mode.run(repo, base, failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(3), "stdout=%s stderr=%s", stdout, stderr)
			Expect(warningLinesNaming(stdout, coachConfigPath)).NotTo(BeEmpty(), "text:\n%s", stdout)
		})

		It("#168: under "+mode.name+", the deleted canonical policy with a root project.json read instead exits 3, because the policy HEAD selects was not read", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
				scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(3), "stdout=%s stderr=%s", stdout, stderr)
			report := decodeCoachReport(stdout)
			expectDiagnosticKind(report, "project_policy_absent_on_disk", coachConfigPath)
			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "project.json was read and applied; the exit is for the policy that was not")
		})

		It("#168: under "+mode.name+", a deleted root project.json that HEAD selects exits 3", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, legacyConfigPath))).To(Succeed())

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(3), "stdout=%s stderr=%s", stdout, stderr)
			expectDiagnosticKind(decodeCoachReport(stdout), "project_policy_absent_on_disk", legacyConfigPath)
		})

		It("#168: under "+mode.name+", a policy on disk that is the policy HEAD selects exits 0 with the flag", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
			expectNoDiagnosticKind(decodeCoachReport(stdout), "project_policy_absent_on_disk")
		})

		It("#168: under "+mode.name+", a repository with no policy committed or on disk exits 0 with the flag", func() {
			repo, base := diskPolicyRepo()

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
			expectNoDiagnosticKind(decodeCoachReport(stdout), "project_policy_absent_on_disk")
		})

		It("#168: under "+mode.name+", an explicit --project-config that names no file still exits 2 with the flag", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})

			stdout, stderr, exitCode := mode.run(repo, base, "--project-config", "missing-policy.json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
		})
	}

	It("#168: docs/cli-codesignal.md names project_policy_absent_on_disk in the exit-code 3 row and in the --fail-on-incomplete-coverage row", func() {
		text := documentText("docs/cli-codesignal.md")

		exitRow := regexp.MustCompile("\\| `3` \\|[^|]*\\|").FindString(text)
		flagRow := regexp.MustCompile("\\| `--fail-on-incomplete-coverage` \\|[^|]*\\|[^|]*\\|").FindString(text)
		Expect(exitRow).To(ContainSubstring("project_policy_absent_on_disk"))
		Expect(flagRow).To(ContainSubstring("project_policy_absent_on_disk"))
	})
})
