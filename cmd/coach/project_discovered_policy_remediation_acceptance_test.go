package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline (TypeScript): a scan whose policy was discovered, not named", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
		commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
		commitFile(repo, coachConfigPath, goLayerPolicyConfigJSON)
		Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
	})

	It("prints a remediation that does not name the discovered policy with --project-config", func() {
		path := pathWithStubNode("v24.9.9")

		stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("typescript_compiler_missing"))
		Expect(string(stderr)).NotTo(ContainSubstring("--project-config"), "naming the discovered policy makes the next command judge it on disk, as an explicit one:\n%s", stderr)
	})

	It("prints no --project-config either when the gap is the host runtime, not the compiler", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, pathWithoutNode(), "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("node_missing"))
		Expect(string(stderr)).NotTo(ContainSubstring("--project-config"), "naming the discovered policy makes the next command judge it on disk, as an explicit one:\n%s", stderr)
	})

	It("prints a check that judges the policy as the scan did, from the analyzed revision", func() {
		path := pathWithStubNode("v24.9.9")
		_, stderr, _ := runCoachCodesignalBaselineEnv(repo, path, "--project-language", "typescript", "--format=json")

		args := codesignalArgsFromRemediationLine(stderr)
		stdout, checkStderr, exitCode := runCoachCheckProjectEnv(repo, path, append(args, "--format", "json")...)

		Expect(exitCode).To(Equal(0), "stderr: %s", checkStderr)
		var doc readinessResultDoc
		Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
		Expect(strings.TrimSpace(doc.Checks.Policy.State)).To(Equal("pass"), "the scan read the committed policy; the printed check must agree: %+v", doc.Checks.Policy)
	})
})
