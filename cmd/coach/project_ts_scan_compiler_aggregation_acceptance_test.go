package main

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --project-language typescript: the scan resolves its compiler through the same aggregation (SA-280-043/044)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
		commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	})

	When("the table reports a compiler pass from a mise origin the project origin could not supply", func() {
		It("never refuses the scan: a readiness pass means the scan can load that same compiler", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.9.3"}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

			checkStdout, checkStderr, checkExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(checkExit).To(Equal(0), "stderr: %s", checkStderr)
			var doc readinessResultDoc
			Expect(json.Unmarshal(checkStdout, &doc)).To(Succeed())
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "this fixture is only meaningful while readiness passes, got code=%s", doc.Checks.Compiler.Code)

			_, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).NotTo(Equal(2), "a passing readiness compiler must never be an unresolved scan-time compiler, stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_compiler_missing"))
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_version_mismatch"))
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_version_conflict"))
		})
	})

	When("the table reports typescript_compiler_missing", func() {
		It("refuses the scan with exit 2 and that same gap code on the D3 stderr line plus AC-SET-9's appended remediation, with no root finding to name", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "no package manager and no mise scope are declared, so the readiness menu offers nothing installable and O2 withholds the appended --prepare-compiler command")
		})
	})

	When("the table reports typescript_version_mismatch for a compiler no selected root resolved", func() {
		It("refuses the scan naming that gap with no root segment, never attributing it to a root that resolved nothing", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"5.9.3\"\n")

			path, _ := pathWithStubMiseDefaultTool("v24.9.9", "5.9.3")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_mismatch: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"),
				"a mismatch a mise origin caused must not attribute the unsupported compiler to a selected root that resolved nothing, and the pinned 5.9.3 is not itself installable so O2 withholds the appended --prepare-compiler command too, got %q", stderr)
		})
	})
})
