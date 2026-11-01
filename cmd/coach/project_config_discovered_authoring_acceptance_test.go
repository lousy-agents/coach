//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	discoveredAuthoringSessionAnswers = "1\n\n\n\napprove\n"
	layerlessOfferText                = "declares no layers"
	layerlessOfferAccepted            = "y\n" + discoveredAuthoringSessionAnswers
	layerlessOfferDeclined            = "n\n"
)

func commitStubToolchainTypeScriptRepo(policyPath, policy string) string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	commitFile(repo, policyPath, policy)
	return repo
}

func commitRealCompilerTypeScriptRepo(policyPath, policy string) string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, policyPath, policy)
	installRealTypescriptCompiler(repo, true)
	return repo
}

func envWithout(name string) []string {
	var env []string
	for _, entry := range os.Environ() {
		if len(entry) > len(name) && entry[:len(name)+1] == name+"=" {
			continue
		}
		env = append(env, entry)
	}
	return env
}

var _ = Describe("coach codesignal guided policy authoring on a discovered .lousy-agents/coach.json (coach#510)", func() {
	When("no --project-config is set, the committed canonical policy is schema-invalid, and a controlling terminal is available", func() {
		It("replaces the scan with guided authoring: exits 2, prints the candidate on stdout, and names .lousy-agents/coach.json as the file to replace", func() {
			repo := commitStubToolchainTypeScriptRepo(coachConfigPath, invalidPolicyText)
			path := pathWithStubNode("v24.9.9")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), discoveredAuthoringSessionAnswers,
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)

			var candidate map[string]any
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout must carry the candidate document; stdout: %s stderr: %s", stdout, stderr)
			Expect(candidate).To(HaveKeyWithValue("schema_version", "1"))
			Expect(candidate).To(HaveKey("roots"))
			Expect(candidate).NotTo(HaveKey("signals"), "no scan report may be produced on this path")

			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "the invalid file must still be reported before the session opens; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(coachConfigPath), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("no file was created"), "a scan writes no repository file; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("replacing"), "the closing instruction must say the candidate replaces the existing file; stderr: %s", stderr)

			onDisk, err := os.ReadFile(filepath.Join(repo, coachConfigPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(onDisk)).To(Equal(invalidPolicyText), "the existing file must never be touched")
		})

		It("exits 2 with empty stdout and leaves the file untouched when the session is declined", func() {
			repo := commitStubToolchainTypeScriptRepo(coachConfigPath, invalidPolicyText)
			path := pathWithStubNode("v24.9.9")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "1\n\n\n\nno\n",
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "declining must never emit a candidate or a report; stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("authoring was cancelled or not approved"), "stderr: %s", stderr)

			onDisk, err := os.ReadFile(filepath.Join(repo, coachConfigPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(onDisk)).To(Equal(invalidPolicyText))
		})
	})

	DescribeTable("the invalid canonical policy never opens an offer when prompts are off, and stays exit 2 with empty stdout",
		func(env func(path string) []string, args []string, withTerminal bool) {
			repo := commitStubToolchainTypeScriptRepo(coachConfigPath, invalidPolicyText)
			path := pathWithStubNode("v24.9.9")
			base := []string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}

			var stdout, stderr []byte
			var exitCode int
			if withTerminal {
				stdout, stderr, _, exitCode = runCoachBinaryWithControllingTerminal(commandPath, repo, env(path), discoveredAuthoringSessionAnswers, append(base, args...)...)
			} else {
				stdout, stderr, exitCode = runCoachBinary(commandPath, repo, env(path), append(base, args...)...)
			}

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(coachConfigPath), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "no authoring session may open; stderr: %s", stderr)
		},
		Entry("no controlling terminal", func(path string) []string { return stubToolchainEnv(path) }, []string{}, false),
		Entry("a controlling terminal under a non-empty CI variable", func(path string) []string { return append(stubToolchainEnv(path), "CI=1") }, []string{}, true),
		Entry("a controlling terminal with --no-interactive", func(path string) []string { return stubToolchainEnv(path) }, []string{"--no-interactive"}, true),
		Entry("a controlling terminal with an explicit --project-config naming the invalid canonical file", func(path string) []string { return stubToolchainEnv(path) }, []string{"--project-config", coachConfigPath}, true),
	)

	When("no --project-config is set, the committed canonical policy is valid but declares no layers, and a controlling terminal is available", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("runs the scan first: stdout carries only the report, the offer goes to stderr, and the exit code follows the scan when the offer is declined", func() {
			repo := commitRealCompilerTypeScriptRepo(coachConfigPath, singleRootPolicyJSON)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, envWithout("CI"), layerlessOfferDeclined,
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(string(stderr)).To(ContainSubstring(layerlessOfferText), "the offer must appear on stderr; stderr: %s", stderr)
		})

		It("keeps stdout a single report and the exit code unchanged when the offer is accepted, printing the approved candidate on stderr and naming the file to replace", func() {
			repo := commitRealCompilerTypeScriptRepo(coachConfigPath, singleRootPolicyJSON)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, envWithout("CI"), layerlessOfferAccepted,
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "the exit code follows the scan; stdout: %s stderr: %s", stdout, stderr)
			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"), "stdout must stay one parseable report; stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring(layerlessOfferText), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`"roots"`), "the approved candidate must be printed on stderr; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(coachConfigPath), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("no file was created"), "stderr: %s", stderr)

			onDisk, err := os.ReadFile(filepath.Join(repo, coachConfigPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(onDisk)).To(Equal(singleRootPolicyJSON), "the existing file must never be touched")
		})
	})

	DescribeTable("a scan with a layerless canonical policy, or a policy the owner did not name, makes no authoring offer",
		func(policyPath, policy string, env func() []string, withTerminal bool) {
			skipWithoutRealTypeScriptCompiler()
			repo := commitRealCompilerTypeScriptRepo(policyPath, policy)
			base := []string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}

			var stdout, stderr []byte
			var exitCode int
			if withTerminal {
				stdout, stderr, _, exitCode = runCoachBinaryWithControllingTerminal(commandPath, repo, env(), "\n", base...)
			} else {
				stdout, stderr, exitCode = runCoachBinary(commandPath, repo, env(), base...)
			}

			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(string(stderr)).NotTo(ContainSubstring(layerlessOfferText), "stderr: %s", stderr)
		},
		Entry("a layerless canonical policy under a non-empty CI variable", coachConfigPath, singleRootPolicyJSON, func() []string { return append(envWithout("CI"), "CI=1") }, true),
		Entry("a layerless canonical policy and no controlling terminal", coachConfigPath, singleRootPolicyJSON, func() []string { return envWithout("CI") }, false),
		Entry("a canonical policy that declares layers", coachConfigPath, goLayerPolicyConfigJSON, func() []string { return envWithout("CI") }, true),
		Entry("a layerless root project.json that is only the deprecated fallback", legacyConfigPath, singleRootPolicyJSON, func() []string { return envWithout("CI") }, true),
	)
})
