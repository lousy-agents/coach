//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
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

const nonCanonicalTerminalTimeout = 15 * time.Second

// runCoachOnNonCanonicalTerminal runs coach on a pseudo-terminal that hands
// bytes over as they arrive, instead of one line per read. Everything typed is
// already waiting when coach starts, so a prompt that reads ahead consumes the
// answers meant for the next prompt, and coach hangs until the timeout.
func runCoachOnNonCanonicalTerminal(dir string, env []string, typed string, args ...string) (stdout, stderr []byte, timedOut bool, err error) {
	GinkgoHelper()
	master, slave := openPTYPair()
	defer master.Close()

	settings, getErr := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	Expect(getErr).NotTo(HaveOccurred())
	settings.Lflag &^= unix.ICANON
	settings.Cc[unix.VMIN] = 1
	settings.Cc[unix.VTIME] = 0
	Expect(unix.IoctlSetTermios(int(slave.Fd()), unix.TCSETS, settings)).To(Succeed())

	ctx, cancel := context.WithTimeout(context.Background(), nonCanonicalTerminalTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, commandPath, args...)
	command.Dir = dir
	command.Env = env
	command.Stdin = slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	_, writeErr := master.WriteString(typed)
	Expect(writeErr).NotTo(HaveOccurred())
	Expect(command.Start()).To(Succeed())
	Expect(slave.Close()).To(Succeed())
	go io.Copy(io.Discard, master)

	err = command.Wait()
	return outBuf.Bytes(), errBuf.Bytes(), ctx.Err() != nil, err
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

	When("the committed canonical policy declares no layers and coverage is incomplete", Label("ts-project-backend"), func() {
		It("offers authoring after a scan that exits 0 and offers nothing after the same scan with --fail-on-incomplete-coverage exits 3", func() {
			skipWithoutRealTypeScriptCompiler()
			repo := newTempGitRepo()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)
			installRealTypescriptCompiler(repo, true)
			scan := []string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}

			By("without the flag the scan exits 0 and the offer appears, which proves this fixture reaches the offer")
			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, envWithout("CI"), layerlessOfferDeclined, scan...)
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			Expect(decodeCoachReport(stdout).ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
			Expect(string(stderr)).To(ContainSubstring(layerlessOfferText), "stderr: %s", stderr)

			By("with the flag the scan exits 3, still renders its report, and makes no offer")
			stdout, stderr, _, exitCode = runCoachBinaryWithControllingTerminal(commandPath, repo, envWithout("CI"), layerlessOfferDeclined, append(scan, "--fail-on-incomplete-coverage")...)
			Expect(exitCode).To(Equal(3), "stdout: %s stderr: %s", stdout, stderr)
			Expect(decodeCoachReport(stdout).ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
			Expect(string(stderr)).NotTo(ContainSubstring(layerlessOfferText), "stderr: %s", stderr)
		})
	})

	When("a Go scan has no --project-config, the committed canonical policy is schema-invalid, and a controlling terminal is available", func() {
		It("opens no TypeScript authoring session and stays exit 2 with empty stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "a.go", minimalGoSource)
			commitFile(repo, coachConfigPath, invalidPolicyText)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, envWithout("CI"), discoveredAuthoringSessionAnswers,
				"codesignal", "--baseline", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("No TypeScript roots"), "stderr: %s", stderr)
		})
	})

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

		It("accepts the offer and completes the session when the terminal delivers typed-ahead input without line buffering", func() {
			repo := commitRealCompilerTypeScriptRepo(coachConfigPath, singleRootPolicyJSON)

			stdout, stderr, timedOut, err := runCoachOnNonCanonicalTerminal(repo, envWithout("CI"), layerlessOfferAccepted,
				"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

			Expect(timedOut).To(BeFalse(), "the offer's answer must not swallow the session's typed-ahead answers; stdout: %s stderr: %s", stdout, stderr)
			Expect(err).NotTo(HaveOccurred(), "stdout: %s stderr: %s", stdout, stderr)
			Expect(decodeCoachReport(stdout).SchemaVersion).To(Equal("2"), "stdout must stay one report")
			Expect(string(stderr)).To(ContainSubstring(`"roots"`), "the approved candidate must be printed; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("no file was created"), "stderr: %s", stderr)
		})
	})

	DescribeTable("a scan with a layerless canonical policy, or a policy the owner did not name, makes no authoring offer",
		Label("ts-project-backend"),
		func(policyPath, policy string, env func() []string, withTerminal bool, args []string) {
			skipWithoutRealTypeScriptCompiler()
			repo := commitRealCompilerTypeScriptRepo(policyPath, policy)
			base := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, args...)

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
		Entry("a layerless canonical policy under a non-empty CI variable", coachConfigPath, singleRootPolicyJSON, func() []string { return append(envWithout("CI"), "CI=1") }, true, nil),
		Entry("a layerless canonical policy and no controlling terminal", coachConfigPath, singleRootPolicyJSON, func() []string { return envWithout("CI") }, false, nil),
		Entry("a layerless canonical policy with --no-interactive", coachConfigPath, singleRootPolicyJSON, func() []string { return envWithout("CI") }, true, []string{"--no-interactive"}),
		Entry("a layerless canonical policy named by an explicit --project-config", coachConfigPath, singleRootPolicyJSON, func() []string { return envWithout("CI") }, true, []string{"--project-config", coachConfigPath}),
		Entry("a canonical policy that declares layers", coachConfigPath, goLayerPolicyConfigJSON, func() []string { return envWithout("CI") }, true, nil),
		Entry("a layerless root project.json that is only the deprecated fallback", legacyConfigPath, singleRootPolicyJSON, func() []string { return envWithout("CI") }, true, nil),
	)
})
