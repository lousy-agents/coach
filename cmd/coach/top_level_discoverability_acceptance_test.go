package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach top-level discoverability", func() {
	When("--help is requested", func() {
		It("exits 0 and writes top-level usage to stdout naming codesignal and its help flag", func() {
			stdout, stderr, exitCode := runCoachRaw("--help")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())
			Expect(string(stdout)).To(ContainSubstring("codesignal"))
			Expect(string(stdout)).To(ContainSubstring("coach codesignal --help"))
		})
	})

	When("-h is requested", func() {
		It("exits 0 and writes top-level usage to stdout naming codesignal and its help flag", func() {
			stdout, stderr, exitCode := runCoachRaw("-h")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())
			Expect(string(stdout)).To(ContainSubstring("codesignal"))
			Expect(string(stdout)).To(ContainSubstring("coach codesignal --help"))
		})
	})

	When("--version is requested", func() {
		It("exits 0 and writes exactly one non-empty version line to stdout", func() {
			stdout, stderr, exitCode := runCoachRaw("--version")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
			Expect(lines).To(HaveLen(1))
			Expect(lines[0]).NotTo(BeEmpty())
		})
	})

	When("coach codesignal --help is requested", func() {
		It("exits 0 and writes flag documentation to stdout", func() {
			stdout, stderr, exitCode := runCoachRaw("codesignal", "--help")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("--base"))
			Expect(text).To(ContainSubstring("--baseline"))
			Expect(text).To(ContainSubstring("--format"))
			Expect(text).To(ContainSubstring("--scope"))
			Expect(text).To(ContainSubstring("--build-target"))
			Expect(text).To(ContainSubstring("--project-config"))
			Expect(text).To(ContainSubstring("--project-language"))
			Expect(text).To(ContainSubstring("--suggest-project-config --project-language typescript"))
		})
	})

	When("coach codesignal -h is requested", func() {
		It("exits 0 and writes flag documentation to stdout", func() {
			stdout, stderr, exitCode := runCoachRaw("codesignal", "-h")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("--base"))
			Expect(text).To(ContainSubstring("--baseline"))
			Expect(text).To(ContainSubstring("--format"))
			Expect(text).To(ContainSubstring("--scope"))
			Expect(text).To(ContainSubstring("--build-target"))
			Expect(text).To(ContainSubstring("--project-config"))
			Expect(text).To(ContainSubstring("--project-language"))
			Expect(text).To(ContainSubstring("--suggest-project-config --project-language typescript"))
		})
	})

	When("an unsupported top-level command is supplied", func() {
		It("exits 2 and writes actionable usage guidance to stderr", func() {
			stdout, stderr, exitCode := runCoachRaw("bogus")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("bogus"))
			Expect(string(stderr)).To(ContainSubstring("codesignal"))
		})
	})
})
