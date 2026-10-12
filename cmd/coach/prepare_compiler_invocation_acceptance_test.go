package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --prepare-compiler --project-language typescript", func() {
	DescribeTable("invalid --prepare-compiler argument combinations exit 2 for the specific reason validatePrepareCompilerFlags reports",
		func(args []string, wantMessage string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, args...)
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring(wantMessage))
		},
		Entry("missing --baseline and --project-language",
			[]string{"--prepare-compiler"},
			"coach: --prepare-compiler requires --baseline"),
		Entry("missing --baseline",
			[]string{"--prepare-compiler", "--project-language", "typescript"},
			"coach: --prepare-compiler requires --baseline"),
		Entry("missing --project-language",
			[]string{"--baseline", "--prepare-compiler"},
			`coach: --prepare-compiler requires --project-language typescript (got "go")`),
		Entry("--project-language go is not typescript",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "go"},
			`coach: --prepare-compiler requires --project-language typescript (got "go")`),
		Entry("duplicate --prepare-compiler",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "--prepare-compiler"},
			"coach: --prepare-compiler may only be provided once"),
		Entry("--prepare-compiler cannot be combined with --check-project",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "--check-project"},
			"coach: --prepare-compiler cannot be combined with --check-project"),
		Entry("--prepare-compiler does not accept positional arguments",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "extra-positional-arg"},
			"coach: --prepare-compiler does not accept positional arguments"),
		Entry("--project-config is an absolute path",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "--project-config", "/etc/passwd"},
			`coach: --project-config "/etc/passwd" is invalid: path must be a non-empty repository-relative path`),
	)

	When("--prepare-compiler is supplied with --baseline and --project-language typescript, but stdin has no controlling terminal", func() {
		It("reaches the real interactive compiler-setup dispatch, which refuses without a controlling terminal, proving the flag is genuinely wired into the compiled binary's flag parser (AC-SET-24)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("no controlling terminal is available"))
			Expect(string(stderr)).To(ContainSubstring("coach codesignal --baseline --prepare-compiler --project-language typescript"))
		})
	})
})
