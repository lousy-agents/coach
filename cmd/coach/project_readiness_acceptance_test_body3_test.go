package main

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectReadinessAcceptanceTest_listsNoMiseInstallationChoiceExits2AndStderrName_1748() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

	path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")
	GinkgoT().Setenv("PATH", path)
	GinkgoT().Setenv("HOME", os.Getenv("HOME"))

	stdin := authoringStdin("")
	defer stdin.Close()
	stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
	defer stdoutFile.Close()
	defer stderrFile.Close()

	exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

	Expect(exitCode).To(Equal(2))
	Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")

	transcript := readStderr()
	Expect(transcript).NotTo(ContainSubstring("mise_project"), "transcript: %s", transcript)
	Expect(transcript).NotTo(ContainSubstring("mise_global"), "transcript: %s", transcript)
	lines := strings.Split(strings.TrimRight(transcript, "\n"), "\n")
	Expect(lines).To(HaveLen(1), "stderr must be exactly one line, transcript: %s", transcript)
	Expect(transcript).To(ContainSubstring("author_policy"), "transcript: %s", transcript)
	Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "compiler setup must never be attempted while a policy gap remains")

	stdout, stderr, exitCodeCheck := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
	Expect(exitCodeCheck).To(Equal(0), "stderr: %s", stderr)
	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
	Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
	var prepareAction, policyAction readinessNextActionDoc
	var foundPrepare, foundPolicy bool
	for _, a := range doc.NextActions {
		if a.Kind == "prepare_compiler" {
			prepareAction, foundPrepare = a, true
		}
		if a.Kind == "author_policy" {
			policyAction, foundPolicy = a, true
		}
	}
	Expect(foundPolicy).To(BeTrue(), "next_actions: %+v", doc.NextActions)
	Expect(foundPrepare).To(BeTrue(), "next_actions: %+v", doc.NextActions)
	Expect(policyAction.Executable).To(BeFalse())
	Expect(prepareAction.Executable).To(BeTrue(), "--check-project's own next_actions must be unaffected by --prepare-compiler's own withholding")
}
