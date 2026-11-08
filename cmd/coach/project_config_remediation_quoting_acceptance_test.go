package main

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal (TypeScript): the other gaps listed beside a missing explicit policy", func() {
	alsoFailingLines := func(policy string) []string {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")

		stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, pathWithStubNode("v24.9.9"), "--project-config", policy, "--project-language", "typescript", "--format=json")

		ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		ExpectWithOffset(1, string(stderr)).NotTo(ContainSubstring("\x1b"), "the escape character reached the terminal:\n%q", stderr)
		var lines []string
		for _, line := range strings.Split(string(stderr), "\n") {
			if strings.Contains(line, "also failing") {
				lines = append(lines, line)
			}
		}
		ExpectWithOffset(1, lines).NotTo(BeEmpty(), "the compiler gap is listed beside the policy gap:\n%s", stderr)
		return lines
	}

	It("quotes a policy path that a shell would expand, so the printed command runs the check and nothing else", func() {
		lines := alsoFailingLines("$(touch PWNED).json")

		Expect(lines[0]).To(HaveSuffix(` --project-config '$(touch PWNED).json'`))
	})

	DescribeTable("quotes a policy path that starts with a tilde, which a shell would replace with a home directory",
		func(policy string) {
			lines := alsoFailingLines(policy)

			Expect(lines[0]).To(HaveSuffix(" --project-config '" + policy + "'"))
		},
		Entry("the current user's home", "~/p.json"),
		Entry("another user's home", "~nobody/p.json"),
	)

	It("leaves a plain policy path as it was", func() {
		lines := alsoFailingLines("policies/p.json")

		Expect(lines[0]).To(HaveSuffix(" --project-config policies/p.json"))
	})

	It("prints no command for a path with a control character, since a command without the flag would check another policy", func() {
		lines := alsoFailingLines("bad\x1bname.json")

		Expect(lines[0]).To(HaveSuffix("check it with --check-project and the same --project-config (its name is not printed here)"))
		Expect(lines[0]).NotTo(ContainSubstring("coach codesignal"), "a command that leaves the flag out runs against the discovered policy")
	})

	It("does not create a file when the printed command is pasted into a shell", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
		_, stderr, _ := runCoachCodesignalBaselineEnv(repo, pathWithStubNode("v24.9.9"), "--project-config", "$(touch PWNED).json", "--project-language", "typescript", "--format=json")
		var printed string
		for _, line := range strings.Split(string(stderr), "\n") {
			if _, command, found := strings.Cut(line, "also failing, run "); found {
				printed = strings.Replace(command, "coach codesignal", "true", 1)
			}
		}
		Expect(printed).NotTo(BeEmpty(), "stderr:\n%s", stderr)

		_, exitCode := runInShell(repo, printed)

		Expect(exitCode).To(Equal(0))
		Expect(filepath.Join(repo, "PWNED")).NotTo(BeAnExistingFile(), "the shell ran the file name as a command")
	})
})
